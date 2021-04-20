package scaleutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodeselector"
	"github.com/hashicorp/nomad/api"
)

// ClusterScaleUtils provides common functionality when performing horizontal
// cluster scaling evaluations and actions.
type ClusterScaleUtils struct {
	log    hclog.Logger
	client *api.Client

	// curNodeID is the ID of the node that the Nomad Autoscaler is currently
	// running on.
	//
	// TODO(jrasell) this should be removed once the cluster targets and core
	//  autoscaler components are updated to handle reconciliation.
	curNodeID string

	// ClusterNodeIDLookupFunc is the callback function used to translate a
	// Nomad nodes ID to the remote resource ID used by the target platform.
	ClusterNodeIDLookupFunc ClusterNodeIDLookupFunc
}

// NewClusterScaleUtils instantiates a new ClusterScaleUtils object for use.
func NewClusterScaleUtils(cfg *api.Config, log hclog.Logger) (*ClusterScaleUtils, error) {

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	// Identifying the node is best-effort and should not result in a terminal
	// error when setting up the utils.
	id, err := autoscalerNodeID(client)
	if err != nil {
		log.Error("failed to identify Nomad Autoscaler nodeID", "error", err)
	}

	return &ClusterScaleUtils{
		log:       log,
		client:    client,
		curNodeID: id,
	}, nil
}

// RunPreScaleInTasks triggers all the tasks, including node identification and
// draining, required before terminating the nodes in the remote provider.
func (c *ClusterScaleUtils) RunPreScaleInTasks(ctx context.Context, cfg map[string]string, num int) ([]NodeResourceID, error) {

	// Check that the ClusterNodeIDLookupFunc has been set, otherwise we cannot
	// attempt to identify nodes and their remote resource IDs.
	if c.ClusterNodeIDLookupFunc == nil {
		return nil, errors.New("required ClusterNodeIDLookupFunc not set")
	}

	nodes, err := c.IdentifyScaleInNodes(cfg, num)
	if err != nil {
		return nil, err
	}

	// Technically we do not need this information until after the nodes have
	// been drained. However, this doesn't change cluster state and so do this
	// first to make sure there are no issues in translating.
	nodeResourceIDs, err := c.IdentifyScaleInRemoteIDs(nodes)
	if err != nil {
		return nil, err
	}

	// Drain the nodes.
	// TODO(jrasell) we should try some reconciliation here, where we identify
	//  failed nodes and continue with nodes that drained successfully.
	if err := c.DrainNodes(ctx, cfg, nodeResourceIDs); err != nil {
		return nil, err
	}
	c.log.Info("pre scale-in tasks now complete")

	return nodeResourceIDs, nil
}

func (c *ClusterScaleUtils) IdentifyScaleInNodes(cfg map[string]string, num int) ([]*api.NodeListStub, error) {

	poolID, err := nodepool.NewClusterNodePoolIdentifier(cfg)
	if err != nil {
		return nil, err
	}
	c.log.Debug("performing node pool filtering", poolID.Key(), poolID.Value())

	// Pull a current list of Nomad nodes from the API including populated
	// resource fields.
	nodes, _, err := c.client.Nodes().List(&api.QueryOptions{Params: map[string]string{"resources": "true"}})
	if err != nil {
		return nil, fmt.Errorf("failed to list Nomad nodes from API: %v", err)
	}

	// Filter our nodes to select only those within our identified pool.
	filteredNodes, err := FilterNodes(nodes, poolID.IsPoolMember)
	if err != nil {
		return nil, err
	}

	// Filter out the Nomad node ID where this autoscaler instance is running.
	filteredNodes = filterOutNodeID(filteredNodes, c.curNodeID)

	// Ensure we have not filtered out all the available nodes.
	if len(filteredNodes) == 0 {
		return nil, fmt.Errorf("no nodes unfiltered for %s with value %s", poolID.Key(), poolID.Value())
	}

	// If the caller has requested more nodes than we have available once
	// filtered, adjust the value. This shouldn't cause the whole scaling
	// action to fail, but we should warn.
	if num > len(filteredNodes) {
		c.log.Warn("can only identify portion of requested nodes for removal",
			"requested", num, "available", len(filteredNodes))
		num = len(filteredNodes)
	}

	// Setup the node selector used to identify suitable nodes for termination.
	selector, err := nodeselector.NewSelector(cfg, c.client, c.log)
	if err != nil {
		return nil, err
	}
	c.log.Debug("performing node selection", "selector_strategy", selector.Name())

	// It is possible the selector is unable to identify suitable nodes and so
	// we should return an error to stop additional execution.
	out := selector.Select(filteredNodes, num)
	if len(out) < 1 {
		return nil, fmt.Errorf("no nodes selected using strategy %s", selector.Name())
	}

	// Selection can filter most of the nodes in the filtered list, we should
	// log about this, but using certain strategies this is to be expected.
	if len(out) < num {
		c.log.Info("identified portion of requested nodes for removal",
			"requested", num, "identified", len(out))
	}
	return out, nil
}

func (c *ClusterScaleUtils) IdentifyScaleInRemoteIDs(nodes []*api.NodeListStub) ([]NodeResourceID, error) {

	var (
		out  []NodeResourceID
		mErr *multierror.Error
	)

	for _, node := range nodes {

		// Read the full node object from the API which will contain the full
		// information required to identify the remote provider ID. If we get a
		// single error here, it's likely we won't be able to perform any of the
		// API calls, therefore just exit rather than collect all the errors.
		nodeInfo, _, err := c.client.Nodes().Info(node.ID, nil)
		if err != nil {
			return nil, err
		}

		// Use the identification function to attempt to pull the remote
		// provider ID information from the Node info.
		id, err := c.ClusterNodeIDLookupFunc(nodeInfo)
		if err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}

		// Add a nice log message for the operators so they can see the node
		// that has been identified if they wish.
		c.log.Debug("identified remote provider ID for node", "node_id", nodeInfo.ID, "remote_id", id)
		out = append(out, NodeResourceID{NomadNodeID: node.ID, RemoteResourceID: id})
	}

	if mErr != nil {
		return nil, errHelper.FormattedMultiError(mErr)
	}
	return out, nil
}

// RunPostScaleInTasks triggers any tasks which should occur after the nodes
// have been terminated within the remote provider.
//
// The context is currently ignored on purpose, pending investigation into
// plugging this into the Nomad API query meta.
func (c *ClusterScaleUtils) RunPostScaleInTasks(_ context.Context, cfg map[string]string, ids []NodeResourceID) error {

	// Attempt to read of the node purge config parameter. If it has been set
	// then check its value, otherwise the default stance is that node purging
	// is disabled.
	if val, ok := cfg[sdk.TargetConfigKeyNodePurge]; ok {

		// Parse the string as a bool. If we get an error return this as the
		// operator has attempted to configure this value, but it's not worth
		// breaking the whole pipeline for. Therefore log the error and return
		// as Nomad will eventually perform this work.
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			c.log.Error("failed to parse node_purge config param", "error", err)
			return nil
		}

		// If the operator has disabled node purging, exit.
		if !boolVal {
			return nil
		}
	} else {
		return nil
	}

	// Use a multierror to collect errors from any and all node purge calls
	// that fail.
	var mErr *multierror.Error

	// Iterate the node list and perform a purge on each node. In the event of
	// an error, add this to the list. Otherwise log useful information.
	for _, node := range ids {

		resp, _, err := c.client.Nodes().Purge(node.NomadNodeID, nil)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			c.log.Info("successfully purged Nomad node", "node_id", node.NomadNodeID, "nomad_evals", resp.EvalIDs)
		}
	}

	return errHelper.FormattedMultiError(mErr)
}

// IsPoolReady provides a method for understanding whether the node pool is in
// a state that allows it to be safely scaled. This should be used by target
// plugins when providing their status response. A non-nil error indicates
// there was a problem performing the check.
func (c *ClusterScaleUtils) IsPoolReady(cfg map[string]string) (bool, error) {

	poolID, err := nodepool.NewClusterNodePoolIdentifier(cfg)
	if err != nil {
		return false, err
	}

	nodes, _, err := c.client.Nodes().List(nil)
	if err != nil {
		return false, fmt.Errorf("failed to list Nomad nodes: %v", err)
	}

	if _, err := FilterNodes(nodes, poolID.IsPoolMember); err != nil {
		c.log.Warn("node pool status readiness check failed", "error", err)
		return false, nil
	}
	return true, nil
}

// autoscalerNodeID identifies the NodeID which the Nomad Autoscaler is running
// on so that it can be protected from scaling in actions.
func autoscalerNodeID(client *api.Client) (string, error) {

	envVar := os.Getenv("NOMAD_ALLOC_ID")
	if envVar == "" {
		return "", nil
	}

	allocInfo, _, err := client.Allocations().Info(envVar, nil)
	if err != nil {
		return "", fmt.Errorf("failed to call Nomad allocation info: %v", err)
	}
	return allocInfo.NodeID, nil
}
