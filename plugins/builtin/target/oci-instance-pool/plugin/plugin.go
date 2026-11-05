// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

const (
	pluginName = "oci-instance-pool"

	configKeyCompartmentID  = "compartment_id"
	configKeyRegion         = "region"
	configKeyInstancePoolID = "instance_pool_id"
	configKeyRetryAttempts  = "retry_attempts"

	// Default ~10 min convergence wait (60 × 10 s); OCI pool ops take 5-10 min.
	configValueRetryAttemptsDefault = "60"

	defaultRetryInterval = 10 * time.Second
)

var (
	// PluginConfig is used by the autoscaler agent to embed the plugin at
	// compile time (internal plugin mode). External deployments ignore this.
	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewOCIInstancePoolPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

var _ target.Target = (*TargetPlugin)(nil)

type TargetPlugin struct {
	config map[string]string
	logger hclog.Logger

	// poolClient talks to the OCI ComputeManagement Instance Pool API.
	// Declared as an interface so tests can inject a mock without a live OCI
	// endpoint.
	poolClient ociComputeManagementClient

	retryAttempts int

	// retryInterval is the wait between polls. Defaults to defaultRetryInterval;
	// overridable in tests to avoid 10-second waits.
	retryInterval time.Duration

	// clusterUtils provides Nomad-side utilities: node-pool readiness check,
	// pre/post scale-in drain/purge tasks, and the remote-ID lookup func.
	clusterUtils *scaleutils.ClusterScaleUtils

	// setupOCIClientFunc is the function used to initialise the OCI client.
	// It is a field so tests can inject a mock.
	setupOCIClientFunc func(config map[string]string) error
}

func NewOCIInstancePoolPlugin(log hclog.Logger) *TargetPlugin {
	p := &TargetPlugin{
		logger:        log,
		retryInterval: defaultRetryInterval,
	}
	p.setupOCIClientFunc = p.setupOCIClient
	return p
}

// Called once per target registration during autoscaler startup.
func (t *TargetPlugin) SetConfig(config map[string]string) error {
	t.config = config

	if err := t.setupOCIClientFunc(config); err != nil {
		return err
	}

	clusterUtils, err := scaleutils.NewClusterScaleUtils(nomad.ConfigFromNamespacedMap(config), t.logger)
	if err != nil {
		return err
	}

	t.clusterUtils = clusterUtils
	t.clusterUtils.ClusterNodeIDLookupFunc = ociNodeIDMap

	retryAttemptsStr := getConfigValue(config, configKeyRetryAttempts, configValueRetryAttemptsDefault)
	attempts, err := strconv.Atoi(retryAttemptsStr)
	if err != nil {
		return fmt.Errorf("invalid value for %s: %v", configKeyRetryAttempts, err)
	}
	t.retryAttempts = attempts

	return nil
}

func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale-in drains/purges selected Nomad nodes then detaches the exact OCI instances,
// rather than decrementing the pool size and letting OCI choose which VMs to evict.
func (t *TargetPlugin) Scale(action sdk.ScalingAction, config map[string]string) error {
	// OCI can't support dry-run like Nomad, so just exit.
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	poolID, err := t.getInstancePoolID(config)
	if err != nil {
		return err
	}

	ctx := context.Background()

	currentCount, _, err := t.poolStatus(ctx, poolID)
	if err != nil {
		return fmt.Errorf("failed to get OCI instance pool status: %w", err)
	}

	num, direction := t.calculateDirection(currentCount, action.Count)

	switch direction {
	case "out":
		err = t.scaleOut(ctx, poolID, action.Count, config)
	case "in":
		err = t.scaleIn(ctx, poolID, num, config)
	default:
		t.logger.Info("scaling not required",
			"instance_pool_id", poolID,
			"current_count", currentCount,
			"strategy_count", action.Count,
		)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to perform scaling action: %w", err)
	}
	return nil
}

// Polled between scaling evaluations.
func (t *TargetPlugin) Status(config map[string]string) (*sdk.TargetStatus, error) {
	// Check Nomad node pool first; if not ready we can skip the OCI API call.
	ready, err := t.clusterUtils.IsPoolReady(config)
	if err != nil {
		return nil, fmt.Errorf("failed to run Nomad node readiness check: %w", err)
	}
	if !ready {
		return &sdk.TargetStatus{Ready: ready}, nil
	}

	poolID, err := t.getInstancePoolID(config)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	count, poolReady, err := t.poolStatus(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("failed to describe OCI instance pool: %w", err)
	}

	return &sdk.TargetStatus{
		Ready: poolReady,
		Count: count,
		Meta:  make(map[string]string),
	}, nil
}

// calculateDirection returns the delta (always positive) and the scaling
// direction ("in", "out", or "" when no change is needed).
func (t *TargetPlugin) calculateDirection(current, desired int64) (int64, string) {
	switch {
	case desired < current:
		return current - desired, "in"
	case desired > current:
		return desired, "out"
	default:
		return 0, ""
	}
}

// getInstancePoolID retrieves the instance_pool_id from the per-policy config
// or falls back to the plugin-level config.
func (t *TargetPlugin) getInstancePoolID(config map[string]string) (string, error) {
	id, ok := t.getValue(config, configKeyInstancePoolID)
	if !ok {
		return "", fmt.Errorf("required config param %s not found", configKeyInstancePoolID)
	}
	return id, nil
}

// getValue retrieves a configuration value from the policy config map first,
// then from the plugin's stored config (plugin-level defaults).
func (t *TargetPlugin) getValue(config map[string]string, name string) (string, bool) {
	if v, ok := config[name]; ok {
		return v, true
	}
	if v, ok := t.config[name]; ok {
		return v, true
	}
	return "", false
}

func getConfigValue(config map[string]string, key, defaultValue string) string {
	if v, ok := config[key]; ok {
		return v
	}
	return defaultValue
}
