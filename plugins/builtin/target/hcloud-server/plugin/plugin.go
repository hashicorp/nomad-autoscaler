package plugin

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

const (
	// pluginName is the unique name of the this plugin amongst Target plugins.
	pluginName = "hcloud-server"

	// configKeys represents the known configuration parameters required at
	// varying points throughout the plugins lifecycle.
	configKeyToken      = "hcloud_token"
	configKeyDatacenter = "hcloud_datacenter"
	configKeyLocation   = "hcloud_location"
	configKeyImage      = "hcloud_image"
	configKeyUserData   = "hcloud_user_data"
	configKeySSHKeys    = "hcloud_ssh_keys"
	configKeyLabels     = "hcloud_labels"
	configKeyServerType = "hcloud_server_type"
	configKeyNamePrefix = "hcloud_name_prefix"
)

var (
	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewHCloudServerPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the Hetzner Cloud Server implementation of the target.Target interface.
type TargetPlugin struct {
	config map[string]string
	logger hclog.Logger
	hcloud *hcloud.Client

	// clusterUtils provides general cluster scaling utilities for querying the
	// state of nodes pools and performing scaling tasks.
	clusterUtils *scaleutils.ClusterScaleUtils
}

// NewHCloudServerPlugin returns the Hetzner Cloud Server implementation of the target.Target
// interface.
func NewHCloudServerPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {

	t.config = config

	if err := t.setupHCloudClient(config); err != nil {
		return err
	}

	clusterUtils, err := scaleutils.NewClusterScaleUtils(nomad.ConfigFromNamespacedMap(config), t.logger)
	if err != nil {
		return err
	}

	// Store and set the remote ID callback function.
	t.clusterUtils = clusterUtils
	t.clusterUtils.ClusterNodeIDLookupFunc = hcloudNodeIDMap

	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale satisfies the Scale function on the target.Target interface.
func (t *TargetPlugin) Scale(action sdk.ScalingAction, config map[string]string) error {

	// AWS can't support dry-run like Nomad, so just exit.
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	namePrefix, ok := config[configKeyNamePrefix]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyNamePrefix)
	}

	labels, ok := config[configKeyLabels]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyLabels)
	}
	ctx := context.Background()

	// Get Hetzner Cloud servers. This serves to both validate the config value is
	// correct and ensure the HCloud client is configured correctly. The response
	// can also be used when performing the scaling, meaning we only need to
	// call it once.
	servers, err := t.getServers(ctx, labels)
	if err != nil {
		return fmt.Errorf("failed to get HCloud servers: %v", err)
	}

	// The Hetzner Cloud servers require different details depending on which
	// direction we want to scale. Therefore calculate the direction and the
	// relevant number so we can correctly perform the HCloud work.
	num, direction := t.calculateDirection(int64(len(servers)), action.Count)

	switch direction {
	case "in":
		err = t.scaleIn(ctx, servers, num, config)
	case "out":
		err = t.scaleOut(ctx, servers, num, config)
	default:
		t.logger.Info("scaling not required", "hcloud_name_prefix", namePrefix,
			"current_count", len(servers), "strategy_count", action.Count)
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

	// Perform our check of the Nomad node pool. If the pool is not ready, we
	// can exit here and avoid calling the AWS API as it won't affect the
	// outcome.
	ready, err := t.clusterUtils.IsPoolReady(config)
	if err != nil {
		return nil, fmt.Errorf("failed to run Nomad node readiness check: %v", err)
	}
	if !ready {
		return &sdk.TargetStatus{Ready: ready}, nil
	}

	labels, ok := config[configKeyLabels]
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyLabels)
	}
	ctx := context.Background()

	servers, err := t.getServers(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of hetzner servers: %v", err)
	}

	serverCount := int64(len(servers))

	// Set our initial status. The asg.Status field is only set when the ASG is
	// being deleted.
	resp := sdk.TargetStatus{
		Ready: serverCount > 0,
		Count: serverCount,
		Meta:  make(map[string]string),
	}

	return &resp, nil
}

func (t *TargetPlugin) calculateDirection(current, strategyDesired int64) (int64, string) {

	if strategyDesired < current {
		return current - strategyDesired, "in"
	}
	if strategyDesired > current {
		return strategyDesired, "out"
	}
	return 0, ""
}
