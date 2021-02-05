package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/actions"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/clusters"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/nodes"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/pkg/errors"
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

	region, ok := config[configKeyRegion]
	if !ok {
		return fmt.Errorf("region must be specified in config")
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return errors.Wrap(err, "unable to authenticate openstack client")
	}

	t.client, err = openstack.NewClusteringV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	if err != nil {
		return errors.Wrap(err, "unable to get openstack senlin client")
	}

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
		return fmt.Errorf("failed to scale out OpenStack Senlin cluster %s: %v", cluster.Name, err)
	}

	if err := t.ensureActionsComplete(ctx, []string{actionID}); err != nil {
		return fmt.Errorf("failed to confirm scale out OpenStack Senlin Cluster %s: %v", cluster.Name, err)
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

	nodeIDs, err := t.getNodeIDs(ids)
	if err != nil {
		return err
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "cluster_name", cluster.Name,
		"nodes", nodeIDs)

	// Delete the desired instances.
	log.Debug("deleting instances from Senlin cluster %s", cluster.Name)

	if err := t.deleteNodes(ctx, nodeIDs); err != nil {
		return fmt.Errorf("failed to scale in OpenStack Senlin Cluster %s: %v", cluster.Name, err)
	}
	log.Info("successfully deleted instances from OpenStack Senlin Cluster %s", cluster.Name)

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
		RemoteProvider: scaleutils.RemoteProviderOpenStackInstanceName,
		NodeIDStrategy: scaleutils.IDStrategyNewestCreateIndex,
	}, nil
}

func (t *TargetPlugin) getNodeIDs(ids []scaleutils.NodeID) ([]string, error) {
	instanceIDs := []string{}

	for _, n := range ids {
		nodeOpts := nodes.ListOpts{
			Name: n.RemoteID,
		}

		allPages, err := nodes.List(t.client, nodeOpts).AllPages()
		if err != nil {
			return nil, err
		}

		allNodes, err := nodes.ExtractNodes(allPages)
		if err != nil {
			return nil, err
		}

		if len(allNodes) != 1 {
			return nil, fmt.Errorf("expected 1 node with name %q, but found %d", n, len(allNodes))
		}

		instanceIDs = append(instanceIDs, allNodes[0].ID)
	}

	return instanceIDs, nil
}

func (t *TargetPlugin) deleteNodes(ctx context.Context, nodeIDs []string) error {
	for _, id := range nodeIDs {
		res := nodes.Delete(t.client, id)

		// manually extract actionID until new gophercloud release
		// that includes change to return ActionResult from Delete
		location := res.Header.Get("Location")
		v := strings.Split(location, "actions/")
		if len(v) < 2 {
			return fmt.Errorf("unable to determine action ID when deleting node %s", id)
		}

		actionID := v[1]

		if err := res.ExtractErr(); err != nil {
			return fmt.Errorf("failed to delete node %s: %v", id, err)
		}

		err := t.ensureActionsComplete(ctx, []string{actionID})
		if err != nil {
			return fmt.Errorf("failed to ensure delete action complete for node %s: %v", id, err)
		}
	}

	return nil

}

func (t *TargetPlugin) describeCluster(clusterName string) (*clusters.Cluster, error) {
	cluster, err := clusters.Get(t.client, clusterName).Extract()
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func (t *TargetPlugin) ensureActionsComplete(ctx context.Context, ids []string) error {
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

			if action.Status == "SUCCEEDED" {
				continue
			}

			if action.Status == "FAILED" || action.Status == "CANCELLED" {
				return true, fmt.Errorf("action %s was unsuccessful: %v", action.ID, action.Status)
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
