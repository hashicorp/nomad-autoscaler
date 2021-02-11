package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"google.golang.org/api/compute/v1"
)

const (
	// pluginName is the unique name of the this plugin amongst Target plugins.
	pluginName = "gce-mig"

	configKeyCredentials = "credentials"
	configKeyProject     = "project"
	configKeyRegion      = "region"
	configKeyZone        = "zone"
	configKeyMIGName     = "mig_name"
)

var (
	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewGCEMIGPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the CGE MIG implementation of the target.Target interface.
type TargetPlugin struct {
	config       map[string]string
	logger       hclog.Logger
	scaleInUtils *scaleutils.ScaleIn

	service *compute.Service
}

// NewGCEMIGPlugin returns the GCE MIG implementation of the target.Target
// interface.
func NewGCEMIGPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {

	t.config = config

	if err := t.setupGCEClients(config); err != nil {
		return err
	}

	utils, err := scaleutils.NewScaleInUtils(nomad.ConfigFromNamespacedMap(config), t.logger)
	if err != nil {
		return err
	}
	t.scaleInUtils = utils

	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale satisfies the Scale function on the target.Target interface.
func (t *TargetPlugin) Scale(action sdk.ScalingAction, config map[string]string) error {

	// GCE can't support dry-run like Nomad, so just exit.
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	migRef, err := t.calculateMIG(config)
	if err != nil {
		return err
	}

	ctx := context.Background()

	_, currentCount, err := t.status(ctx, migRef)
	if err != nil {
		return fmt.Errorf("failed to describe GCE Managed Instance Group: %v", err)
	}

	num, direction := t.calculateDirection(currentCount, action.Count)

	switch direction {
	case "in":
		err = t.scaleIn(ctx, migRef, num, config)
	case "out":
		err = t.scaleOut(ctx, migRef, num)
	default:
		t.logger.Info("scaling not required", "mig_name", migRef.getName(),
			"current_count", currentCount, "strategy_count", action.Count)
		return nil
	}

	// If we received an error while scaling, format this with an outer message
	// so its nice for the operators and then return any error to the caller.
	if err != nil {
		err = fmt.Errorf("failed to perform scaling action: %v", err)
	}
	return err
}

// Status satisfies the Status function on the target.Target interface.
func (t *TargetPlugin) Status(config map[string]string) (*sdk.TargetStatus, error) {

	class, ok := config[sdk.TargetConfigKeyClass]
	if !ok {
		return nil, fmt.Errorf("required config param %q not found", sdk.TargetConfigKeyClass)
	}

	// Perform our check of the Nomad node pool. If the pool is not ready, we
	// can exit here and avoid calling the Google API as it won't affect the
	// outcome.
	ready, err := t.scaleInUtils.Ready(scaleutils.PoolIdentifier{
		IdentifierKey: scaleutils.IdentifierKeyClass,
		Value:         class,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run Nomad node readiness check: %v", err)
	}
	if !ready {
		return &sdk.TargetStatus{Ready: ready}, nil
	}

	group, err := t.calculateMIG(config)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	stable, currentCount, err := t.status(ctx, group)
	if err != nil {
		return nil, fmt.Errorf("failed to describe GCE Managed Instance Group: %v", err)
	}

	resp := sdk.TargetStatus{
		Ready: stable,
		Count: currentCount,
		Meta:  make(map[string]string),
	}

	return &resp, nil
}

func (t *TargetPlugin) calculateDirection(migTarget, strategyDesired int64) (int64, string) {
	if strategyDesired < migTarget {
		return migTarget - strategyDesired, "in"
	}
	if strategyDesired > migTarget {
		return strategyDesired, "out"
	}
	return 0, ""
}

func (t *TargetPlugin) calculateMIG(config map[string]string) (instanceGroup, error) {

	// We cannot scale an MIG without knowing the project.
	project, ok := t.getValue(config, configKeyProject)
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyProject)
	}

	// We cannot scale an MIG without knowing the MIG region or zone.
	region, regionOk := t.getValue(config, configKeyRegion)
	zone, zoneOk := t.getValue(config, configKeyZone)
	if !regionOk && !zoneOk {
		return nil, fmt.Errorf("required config param %s or %s not found", configKeyRegion, configKeyZone)
	}

	// We cannot scale an MIG without knowing the MIG name.
	migName, ok := config[configKeyMIGName]
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyMIGName)
	}

	if len(zone) != 0 {
		return &zonalInstanceGroup{
			project: project,
			zone:    zone,
			name:    migName,
		}, nil
	} else {
		return &regionalInstanceGroup{
			project: project,
			region:  region,
			name:    migName,
		}, nil
	}
}

func (t *TargetPlugin) getValue(config map[string]string, name string) (string, bool) {
	v, ok := config[name]
	if ok {
		return v, true
	}

	v, ok = t.config[name]
	if ok {
		return v, true
	}

	return "", false
}
