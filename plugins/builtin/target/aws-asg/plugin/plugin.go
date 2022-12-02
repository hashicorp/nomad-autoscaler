package plugin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

const (
	// pluginName is the unique name of the this plugin amongst Target plugins.
	pluginName = "aws-asg"

	// configKeys represents the known configuration parameters required at
	// varying points throughout the plugins lifecycle.
	configKeyRegion             = "aws_region"
	configKeyAccessID           = "aws_access_key_id"
	configKeySecretKey          = "aws_secret_access_key"
	configKeySessionToken       = "aws_session_token"
	configKeyASGName            = "aws_asg_name"
	configKeyCredentialProvider = "aws_credential_provider"
	configKeyRetryAttempts      = "retry_attempts"

	// configValues are the default values used when a configuration key is not
	// supplied by the operator that are specific to the plugin.
	configValueRegionDefault        = "us-east-1"
	configValueRetryAttemptsDefault = "15"

	// credentialProvider are the valid options for the aws_credential_provider
	// configuration key.
	credentialProviderEC2Role = "ec2_role"
)

var (
	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewAWSASGPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the AWS ASG implementation of the target.Target interface.
type TargetPlugin struct {
	config map[string]string
	logger hclog.Logger
	asg    *autoscaling.Client

	// retryAttempts is the number of times operations such as wating for a
	// given ASG state should be retried.
	retryAttempts int

	// clusterUtils provides general cluster scaling utilities for querying the
	// state of nodes pools and performing scaling tasks.
	clusterUtils *scaleutils.ClusterScaleUtils
}

// NewAWSASGPlugin returns the AWS ASG implementation of the target.Target
// interface.
func NewAWSASGPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {

	t.config = config

	if err := t.setupAWSClients(config); err != nil {
		return err
	}

	clusterUtils, err := scaleutils.NewClusterScaleUtils(nomad.ConfigFromNamespacedMap(config), t.logger)
	if err != nil {
		return err
	}

	// Store and set the remote ID callback function.
	t.clusterUtils = clusterUtils
	t.clusterUtils.ClusterNodeIDLookupFunc = awsNodeIDMap

	retryLimit, err := strconv.Atoi(getConfigValue(config, configKeyRetryAttempts, configValueRetryAttemptsDefault))
	if err != nil {
		return err
	}
	t.retryAttempts = retryLimit

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

	// We cannot scale an ASG without knowing the ASG name.
	asgName, ok := config[configKeyASGName]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyASGName)
	}
	ctx := context.Background()

	// Describe the ASG. This serves to both validate the config value is
	// correct and ensure the AWS client is configured correctly. The response
	// can also be used when performing the scaling, meaning we only need to
	// call it once.
	curASG, err := t.describeASG(ctx, asgName)
	if err != nil {
		return fmt.Errorf("failed to describe AWS Autoscaling Group: %v", err)
	}

	// Autoscaling can interfere with a running instance refresh so we
	// prevent any scaling action while a refresh is Pending or InProgress
	input := autoscaling.DescribeInstanceRefreshesInput{
		AutoScalingGroupName: &asgName,
		MaxRecords:           ptr.Int32ToPtr(1),
	}

	refreshes, err := t.asg.DescribeInstanceRefreshes(ctx, &input)
	if err != nil {
		return fmt.Errorf("failed to describe AWS InstanceRefresh: %v", err)
	}

	for _, refresh := range refreshes.InstanceRefreshes {
		active := refresh.Status == types.InstanceRefreshStatusInProgress ||
			refresh.Status == types.InstanceRefreshStatusPending

		if active {
			t.logger.Warn("scaling will not take place due to InstanceRefresh",
				"asg_name", asgName,
				"refresh_id", refresh.InstanceRefreshId,
				"refresh_status", refresh.Status)
			return nil
		}
	}

	// The AWS ASG target requires different details depending on which
	// direction we want to scale. Therefore calculate the direction and the
	// relevant number so we can correctly perform the AWS work.
	num, direction := t.calculateDirection(int64(*curASG.DesiredCapacity), action.Count)

	switch direction {
	case "in":
		err = t.scaleIn(ctx, curASG, num, config)
	case "out":
		err = t.scaleOut(ctx, curASG, num)
	default:
		t.logger.Info("scaling not required", "asg_name", asgName,
			"current_count", *curASG.DesiredCapacity, "strategy_count", action.Count)
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

	// We cannot get the status of an ASG if we don't know its name.
	asgName, ok := config[configKeyASGName]
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyASGName)
	}
	ctx := context.Background()

	asg, err := t.describeASG(ctx, asgName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AWS Autoscaling Group: %v", err)
	}

	events, err := t.describeActivities(ctx, asgName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AWS Autoscaling Group activities: %v", err)
	}

	// Set our initial status. The asg.Status field is only set when the ASG is
	// being deleted.
	resp := sdk.TargetStatus{
		Ready: asg.Status == nil,
		Count: int64(*asg.DesiredCapacity),
		Meta:  make(map[string]string),
	}

	// If we have previous activities then process the last.
	if len(events) > 0 {
		processLastActivity(events[0], &resp)
	}

	return &resp, nil
}

func (t *TargetPlugin) calculateDirection(asgDesired, strategyDesired int64) (int64, string) {

	if strategyDesired < asgDesired {
		return asgDesired - strategyDesired, "in"
	}
	if strategyDesired > asgDesired {
		return strategyDesired, "out"
	}
	return 0, ""
}

// processLastActivity updates the status object based on the details within
// the last scaling activity.
func processLastActivity(activity types.Activity, status *sdk.TargetStatus) {

	// If the last activities progress is not nil then check whether this
	// finished or not. In the event there is a current activity in progress
	// set ready to false so the autoscaler will not perform any actions.
	if activity.Progress != 100 {
		status.Ready = false
	}

	// EndTime isn't always populated, especially if the activity has not yet
	// finished :).
	if activity.EndTime != nil {
		status.Meta[sdk.TargetStatusMetaKeyLastEvent] = strconv.FormatInt(activity.EndTime.UnixNano(), 10)
	}
}

func getConfigValue(config map[string]string, key string, defaultValue string) string {
	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	return value
}
