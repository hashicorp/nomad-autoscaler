package plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

const (
	defaultRetryInterval = 10 * time.Second
	defaultRetryLimit    = 15
)

// setupAWSClients takes the passed config mapping and instantiates the
// required AWS service clients.
func (t *TargetPlugin) setupAWSClients(config map[string]string) error {

	// Load our default AWS config. This handles pulling configuration from
	// default profiles and environment variables.
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return fmt.Errorf("failed to load default AWS config: %v", err)
	}

	// Check for a configured region and set the value to our internal default
	// if nothing is found.
	region, ok := config[configKeyRegion]
	if !ok {
		region = configValueRegionDefault
	}

	// If the default config is empty, update it.
	if cfg.Region == "" {
		t.logger.Trace("setting AWS region for client", "region", region)
		cfg.Region = region
	}

	// Attempt to pull access credentials for the AWS client from the user
	// supplied configuration. In order to use these static credentials both
	// the access key and secret key need to be present; the session token is
	// optional.
	keyID, idOK := config[configKeyAccessID]
	secretKey, keyOK := config[configKeySecretKey]
	session := config[configKeySessionToken]

	if idOK && keyOK {
		t.logger.Trace("setting AWS access credentials from config map")
		cfg.Credentials = aws.NewStaticCredentialsProvider(keyID, secretKey, session)
	}

	// Set up our AWS clients.
	t.ec2 = ec2.New(cfg)
	t.asg = autoscaling.New(cfg)

	return nil
}

// scaleOut updates the Auto Scaling Group desired count to match what the
// Autoscaler has deemed required.
func (t *TargetPlugin) scaleOut(ctx context.Context, asg *autoscaling.AutoScalingGroup, count int64) error {

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_out", "asg_name", *asg.AutoScalingGroupName,
		"desired_count", count)

	input := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: asg.AutoScalingGroupName,
		AvailabilityZones:    asg.AvailabilityZones,
		DesiredCapacity:      aws.Int64(count),
	}

	// Ignore the response from Send() as its empty.
	_, err := t.asg.UpdateAutoScalingGroupRequest(&input).Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to update Autoscaling Group: %v", err)
	}

	if err := t.ensureASGInstancesCount(ctx, count, *asg.AutoScalingGroupName); err != nil {
		return fmt.Errorf("failed to confirm scale out AWS AutoScaling Group: %v", err)
	}

	log.Info("successfully performed and verified scaling out")
	return nil
}

func (t *TargetPlugin) scaleIn(ctx context.Context, asg *autoscaling.AutoScalingGroup, num int64, config map[string]string) error {

	scaleReq, err := t.generateScaleReq(num, config)
	if err != nil {
		return fmt.Errorf("failed to generate scale in request: %v", err)
	}

	ids, err := t.scaleInUtils.RunPreScaleInTasks(ctx, scaleReq)
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
	}

	// Grab the instanceIDs once as it is used multiple times throughout the
	// scale in event.
	var instanceIDs []string

	for _, node := range ids {
		instanceIDs = append(instanceIDs, node.RemoteID)
	}

	// Create the event writer and write that the drain event has been
	// completed which is part of the RunPreScaleInTasks() function.
	eWriter := newEventWriter(t.logger, t.asg, instanceIDs, *asg.AutoScalingGroupName)
	eWriter.write(ctx, scalingEventDrain)

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "asg_name", *asg.AutoScalingGroupName,
		"instances", instanceIDs)

	// Detach the desired instances.
	log.Debug("detaching instances from AutoScaling Group")

	if err := t.detachInstances(ctx, asg.AutoScalingGroupName, instanceIDs); err != nil {
		return fmt.Errorf("failed to scale in AWS AutoScaling Group: %v", err)
	}
	log.Info("successfully detached instances from AutoScaling Group")
	eWriter.write(ctx, scalingEventDetach)

	// Terminate the detached instances.
	log.Debug("terminating EC2 instances")

	if err := t.terminateInstances(ctx, instanceIDs); err != nil {
		return fmt.Errorf("failed to scale in AWS AutoScaling Group: %v", err)
	}
	log.Info("successfully terminated EC2 instances")
	eWriter.write(ctx, scalingEventTerminate)

	// Run any post scale in tasks that are desired.
	if err := t.scaleInUtils.RunPostScaleInTasks(config, ids); err != nil {
		return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
	}

	return nil
}

func (t *TargetPlugin) generateScaleReq(num int64, config map[string]string) (*scaleutils.ScaleInReq, error) {

	// Pull the class key from the config mapping. This is a required value and
	// we cannot scale without this.
	class, ok := config[sdk.TargetConfigKeyClass]
	if !ok {
		return nil, fmt.Errorf("required config param %q not found", sdk.TargetConfigKeyClass)
	}

	// The drain_deadline is an optional parameter so define out default and
	// then attempt to find an operator specified value.
	drain := scaleutils.DefaultDrainDeadline
	ignoreSystemJobs := scaleutils.DefaultIgnoreSystemJobs

	if drainString, ok := config[sdk.TargetConfigKeyDrainDeadline]; ok {
		d, err := time.ParseDuration(drainString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as time duration", drainString)
		}
		drain = d
	}

	if ignoreSystemJobsString, ok := config[sdk.TargetConfigKeyDrainDeadline]; ok {
		isj, err := strconv.ParseBool(ignoreSystemJobsString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as boolean", ignoreSystemJobsString)
		}
		ignoreSystemJobs = isj
	}

	return &scaleutils.ScaleInReq{
		Num:              int(num),
		DrainDeadline:    drain,
		IgnoreSystemJobs: ignoreSystemJobs,

		PoolIdentifier: &scaleutils.PoolIdentifier{
			IdentifierKey: scaleutils.IdentifierKeyClass,
			Value:         class,
		},
		RemoteProvider: scaleutils.RemoteProviderAWSInstanceID,
		NodeIDStrategy: scaleutils.IDStrategyNewestCreateIndex,
	}, nil
}

func (t *TargetPlugin) detachInstances(ctx context.Context, asgName *string, instanceIDs []string) error {

	asgInput := autoscaling.DetachInstancesInput{
		AutoScalingGroupName:           asgName,
		InstanceIds:                    instanceIDs,
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	}

	asgResp, err := t.asg.DetachInstancesRequest(&asgInput).Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to detach intances from Autoscaling Group: %v", err)
	}

	// Identify the activities that were created as a result of the detachment
	// request so that we can go ahead and track these to completion.
	var activityIDs []string

	for _, activity := range asgResp.Activities {
		activityIDs = append(activityIDs, *activity.ActivityId)
	}

	// Confirm that the detachments complete before moving on. I (jrasell) am
	// not exactly sure what happens if we terminate an instance which is still
	// detaching from an ASG, but we might as well avoid finding out if we can.
	err = t.ensureActivitiesComplete(ctx, activityIDs, *asgName)
	if err != nil {
		return fmt.Errorf("failed to detached instances from AutoScaling Group: %v", err)
	}
	return nil
}

func (t *TargetPlugin) terminateInstances(ctx context.Context, instanceIDs []string) error {

	ec2Input := ec2.TerminateInstancesInput{InstanceIds: instanceIDs}

	// TODO(jrasell) the response includes information about instance status
	//  changes which we may want to validate in the future.
	_, err := t.ec2.TerminateInstancesRequest(&ec2Input).Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to terminate EC2 intances: %v", err)
	}

	// Confirm that the instances have indeed terminated properly. This allows
	// us to handle reconciliation if the error is transient, or at least
	// allows operators to see the error and perform manual actions to resolve.
	err = t.ensureInstancesTerminate(ctx, instanceIDs)
	if err != nil {
		return fmt.Errorf("failed to terminate EC2 instances: %v", err)
	}
	return nil
}

func (t *TargetPlugin) describeASG(ctx context.Context, asgName string) (*autoscaling.AutoScalingGroup, error) {

	input := autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []string{asgName}}

	resp, err := t.asg.DescribeAutoScalingGroupsRequest(&input).Send(ctx)
	if err != nil {
		return nil, err
	}

	if len(resp.AutoScalingGroups) != 1 {
		return nil, fmt.Errorf("expected 1 Autoscaling Group, got %v", len(resp.AutoScalingGroups))
	}
	return &resp.AutoScalingGroups[0], nil
}

func (t *TargetPlugin) describeActivities(ctx context.Context, asgName string, ids []string) ([]autoscaling.Activity, error) {

	input := autoscaling.DescribeScalingActivitiesInput{AutoScalingGroupName: aws.String(asgName)}

	// If an ID is specified, add this to the request so we only pull
	// information regarding this.
	if len(ids) > 0 {
		input.ActivityIds = ids
	}

	resp, err := t.asg.DescribeScalingActivitiesRequest(&input).Send(ctx)
	if err != nil {
		return nil, err
	}

	// If the caller passed a list of IDs to describe, ensure the returned list
	// is the current length.
	if len(ids) > 0 && len(resp.Activities) != len(ids) {
		return nil, fmt.Errorf("expected %v activities, got %v", len(ids), len(resp.Activities))
	}
	return resp.Activities, nil
}

func (t *TargetPlugin) ensureActivitiesComplete(ctx context.Context, ids []string, asg string) error {

	f := func(ctx context.Context) (bool, error) {

		activities, err := t.describeActivities(ctx, asg, ids)
		if err != nil {
			return true, err
		}

		// Reset the scaling activity IDs we are waiting to complete so we can
		// re-populate with a modified list later.
		ids = []string{}

		// Iterate each activity, check the progress and add any incomplete
		// activities to the ID list for rechecking.
		for _, activity := range activities {
			if *activity.Progress != 100 {
				ids = append(ids, *activity.ActivityId)
			}
		}

		// If we dont have any remaining IDs to check, we can finish.
		if len(ids) == 0 {
			return true, nil
		}
		return false, fmt.Errorf("waiting for %v activities to finish", len(ids))
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func (t *TargetPlugin) ensureInstancesTerminate(ctx context.Context, ids []string) error {

	f := func(ctx context.Context) (bool, error) {

		input := ec2.DescribeInstanceStatusInput{InstanceIds: ids}

		resp, err := t.ec2.DescribeInstanceStatusRequest(&input).Send(ctx)
		if err != nil {
			return true, err
		}

		// Reset the instance IDs we want to check so this can be populated again
		// once we have processed their current status information.
		ids = []string{}

		for _, instanceStatus := range resp.InstanceStatuses {
			if instanceStatus.InstanceState.Name != ec2.InstanceStateNameTerminated {
				ids = append(ids, *instanceStatus.InstanceId)
			}
		}

		// If we dont have any remaining IDs to check, we can finish.
		if len(ids) == 0 {
			return true, nil
		}
		return false, fmt.Errorf("waiting for %v instances to terminate", len(ids))
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func (t *TargetPlugin) ensureASGInstancesCount(ctx context.Context, desired int64, asgName string) error {

	f := func(ctx context.Context) (bool, error) {
		asg, err := t.describeASG(ctx, asgName)
		if err != nil {
			return true, err
		}

		if len(asg.Instances) == int(desired) {
			return true, nil
		}
		return false, fmt.Errorf("AutoScaling Group at %v instances of desired %v", asg.Instances, desired)
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}
