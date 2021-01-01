package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/actions"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/clusters"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

const (
	defaultRetryInterval = 10 * time.Second
	defaultRetryLimit    = 15
)

// setupOSClients takes the passed config mapping and instantiates the
// required OpenStack service clients.
func (t *TargetPlugin) setupOSClients(config map[string]string) error {

	// Load our default OpenStack config. This handles pulling configuration from
	// default profiles and environment variables.
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load default OpenStack config: %v", err)
	}

	// Check for a configured region and set the value to our internal default
	// if nothing is found.
	region, ok := config[configKeyRegion]
	if !ok {
		region = configValueRegionDefault
	}

	username, userOK := config[configKeyUserName]
	password, pwOK := config[configKeyPassword]

	if userOK && pwOK {
		t.logger.Trace("setting OpenStack access credentials from config map")

		opts = gophercloud.AuthOptions{
			IdentityEndpoint: "https://openstack.example.com:5000/v2.0",
			Username:         username,
			Password:         password,
		}
	}

	provider, err := openstack.AuthenticatedClient(opts)

	t.client, err = openstack.NewClusteringV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})

	return nil
}

// scaleOut updates the Cluster desired count to match what the
// Autoscaler has deemed required.
func (t *TargetPlugin) scaleOut(ctx context.Context, cluster *clusters.Cluster, count int64) error {

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_out", "cluster_name", cluster.Name,
		"desired_count", count)

	opts := clusters.ScaleOutOpts{
		Count: int(count),
	}

	actionID, err := clusters.ScaleOut(t.client, cluster.ID, opts).Extract()
	if err != nil {
		return fmt.Errorf("failed to scale out Senlin cluster: %v", err)
	}

	if err := t.ensureClusterInstancesCount(ctx, count, cluster.Name, actionID); err != nil {
		return fmt.Errorf("failed to confirm scale out OpenStack Senlin Cluster: %v", err)
	}

	log.Info("successfully performed and verified scaling out")
	return nil
}

func (t *TargetPlugin) scaleIn(ctx context.Context, cluster *clusters.Cluster, num int64, config map[string]string) error {

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
	eWriter := newEventWriter(t.logger, t.client, instanceIDs, cluster.Name)
	eWriter.write(ctx, scalingEventDrain)

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "cluster_name", cluster.Name,
		"instances", instanceIDs)

	// Detach the desired instances.
	log.Debug("detaching instances from Senlin cluster")

	if err := t.detachInstances(ctx, cluster.Name, instanceIDs); err != nil {
		return fmt.Errorf("failed to scale in OpenStack Senlin Cluster: %v", err)
	}
	log.Info("successfully detached instances from OpenStack Senlin Cluster")
	eWriter.write(ctx, scalingEventDetach)

	// Terminate the detached instances.
	log.Debug("terminating VM instances")

	if err := t.terminateInstances(ctx, instanceIDs); err != nil {
		return fmt.Errorf("failed to scale in OpenStack Senlin Cluster: %v", err)
	}
	log.Info("successfully terminated VM instances")
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

	if drainString, ok := config[sdk.TargetConfigKeyDrainDeadline]; ok {
		d, err := time.ParseDuration(drainString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as time duration", drainString)
		}
		drain = d
	}

	return &scaleutils.ScaleInReq{
		Num:           int(num),
		DrainDeadline: drain,
		PoolIdentifier: &scaleutils.PoolIdentifier{
			IdentifierKey: scaleutils.IdentifierKeyClass,
			Value:         class,
		},
		RemoteProvider: scaleutils.RemoteProviderAWSInstanceID,
		NodeIDStrategy: scaleutils.IDStrategyNewestCreateIndex,
	}, nil
}

func (t *TargetPlugin) detachInstances(ctx context.Context, clusterID string, instanceIDs []string) error {
	opts := clusters.RemoveNodesOpts{
		Nodes: instanceIDs,
	}

	err := clusters.RemoveNodes(t.client, clusterID, opts).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to detach instances from OpenStack Senlin Cluster: %v", err)
	}

	/*
		// TODO: fix gophercloud so that RemoveNodes returns actionID to track

		// Identify the activities that were created as a result of the detachment
		// request so that we can go ahead and track these to completion.
		var activityIDs []string

		for _, activity := range asgResp.Activities {
			activityIDs = append(activityIDs, *activity.ActivityId)
		}

		// Confirm that the detachments complete before moving on. I (jrasell) am
		// not exactly sure what happens if we terminate an instance which is still
		// detaching from an ASG, but we might as well avoid finding out if we can.
		err = t.ensureActionsComplete(ctx, activityIDs, *asgName)
		if err != nil {
			return fmt.Errorf("failed to detached instances from AutoScaling Group: %v", err)
		}

	*/

	return nil

}

func (t *TargetPlugin) terminateInstances(ctx context.Context, instanceIDs []string) error {
	for _, i := range instanceIDs {
		err := startstop.Stop(t.client, i).ExtractErr()
		if err != nil {
			return fmt.Errorf("failed to terminate VM instances: %v", err)
		}
	}

	// Confirm that the instances have indeed terminated properly. This allows
	// us to handle reconciliation if the error is transient, or at least
	// allows operators to see the error and perform manual actions to resolve.
	err := t.ensureInstancesTerminate(ctx, instanceIDs)
	if err != nil {
		return fmt.Errorf("failed to terminate VM instances: %v", err)
	}
	return nil
}

func (t *TargetPlugin) describeCluster(ctx context.Context, clusterName string) (*clusters.Cluster, error) {
	cluster, err := clusters.Get(t.client, clusterName).Extract()
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func (t *TargetPlugin) describeActions(ctx context.Context, clusterID string, ids []string) ([]actions.Action, error) {
	if len(ids) > 0 {
		return nil, fmt.Errorf("TODO: implement filtering of action list by given ids")
	}

	opts := actions.ListOpts{
		Target: clusterID,
	}

	resp, err := actions.List(t.client, opts).AllPages()
	if err != nil {
		return nil, err
	}

	actions, err := actions.ExtractActions(resp)
	if err != nil {
		return nil, err
	}

	return actions, nil
}

func (t *TargetPlugin) ensureActionsComplete(ctx context.Context, ids []string, clusterID string) error {

	f := func(ctx context.Context) (bool, error) {
		// Reset the scaling action IDs we are waiting to complete so we can
		// re-populate with a modified list later.
		newIDs := []string{}

		// Iterate each action, check the progress and add any incomplete
		// actions to the ID list for rechecking.
		for _, actionID := range ids {
			action, err := actions.Get(t.client, actionID).Extract()
			if err != nil {
				return true, err
			}

			if action.Status == "SUCCESS" {
				continue
			}

			if action.Status == "FAILED" || action.Status == "CANCELLED" {
				return true, fmt.Errorf("Action %s was unsuccessful: %v", action.ID, action.Status)
			}

			newIDs = append(newIDs, actionID)

		}

		ids = newIDs

		// If we dont have any remaining IDs to check, we can finish.
		if len(ids) == 0 {
			return true, nil
		}

		return false, fmt.Errorf("waiting for %v actions to finish", len(ids))
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func (t *TargetPlugin) ensureInstancesTerminate(ctx context.Context, ids []string) error {

	f := func(ctx context.Context) (bool, error) {
		newIDs := []string{}

		for _, i := range ids {
			s, err := servers.Get(t.client, i).Extract()
			if err != nil {
				return true, err
			}

			if s.Status == "SHUTDOWN" {
				continue
			}

			newIDs = append(newIDs, s.ID)
		}

		ids = newIDs

		// If we dont have any remaining IDs to check, we can finish.
		if len(ids) == 0 {
			return true, nil
		}

		return false, fmt.Errorf("waiting for %v instances to terminate", len(ids))
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func (t *TargetPlugin) ensureClusterInstancesCount(ctx context.Context, desired int64, clusterName string, actionID string) error {

	f := func(ctx context.Context) (bool, error) {
		action, err := actions.Get(t.client, actionID).Extract()
		if err != nil {
			return true, err
		}

		if action.Status == "SUCCESS" {
			return true, nil
		}

		if action.Status == "FAILED" || action.Status == "CANCELLED" {
			return true, fmt.Errorf("Cluster action was unsuccessful: %v", action.Status)
		}

		return false, fmt.Errorf("Action has not completed")
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}
