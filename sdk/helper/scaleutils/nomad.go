package scaleutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

type ScaleIn struct {
	log   hclog.Logger
	nomad *api.Client

	// curNodeID is the ID of the node that the Nomad autoscaler is curently
	// running on.
	//
	// TODO(jrasell) this should be removed once the cluster targets and core
	//  autoscaler components are updated to handle reconciliation.
	curNodeID string
}

// NewScaleInUtils returns a new ScaleIn implementation which provides helper
// functions for performing scaling in operations.
func NewScaleInUtils(cfg *api.Config, log hclog.Logger) (*ScaleIn, error) {

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	// Identifying the node is best-effort and should not result in a terminal
	// error when setting up the utils.
	id, err := identifyAutoscalerNodeID(client)
	if err != nil {
		log.Error("failed to identify Nomad Autoscaler nodeID", "error", err)
	}

	return &ScaleIn{
		log:       log,
		nomad:     client,
		curNodeID: id,
	}, nil
}

// RunPreScaleInTasks helps tie together all the tasks required prior to
// scaling in Nomad nodes, and thus terminating the server in the remote
// provider.
func (si *ScaleIn) RunPreScaleInTasks(ctx context.Context, req *ScaleInReq) ([]NodeID, error) {

	if err := req.validate(); err != nil {
		return nil, fmt.Errorf("failed to validate request: %v", err)
	}

	nodes, err := si.identifyTargets(req.Num, req.PoolIdentifier, req.NodeIDStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to identify nodes for removal: %v", err)
	}

	// Technically we do not need this information until after the nodes have
	// been drained. However, this doesn't change cluster state and so do this
	// first to make sure there are no issues in translating.
	nodeIDMap, err := si.getRemoteIDMap(nodes, req.RemoteProvider)
	if err != nil {
		return nil, err
	}

	// If we have not been able to identify any nodes and get their remote
	// provider ID we cannot continue.
	if len(nodeIDMap) == 0 {
		return nil, errors.New("failed to identify nodes for removal")
	}

	if err := si.drainNodes(ctx, req.IgnoreSystemJobs, req.DrainDeadline, nodeIDMap); err != nil {
		return nil, err
	}

	return nodeIDMap, nil
}

// RunPostScaleInTasks runs any tasks that need to occur after a remote node
// provider has completed its work. It handles any users configuration so that
// the plugin does not need to perform this work.
func (si *ScaleIn) RunPostScaleInTasks(cfg map[string]string, nodes []NodeID) error {

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
			si.log.Error("failed to parse node_purge config param", "error", err)
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
	for _, node := range nodes {
		resp, _, err := si.nomad.Nodes().Purge(node.NomadID, nil)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			si.log.Info("successfully purged Nomad node",
				"node_id", node.NomadID, "nomad_evals", resp.EvalIDs)
		}
	}

	return mErr.ErrorOrNil()
}

// identifyTargets filters the current Nomad cluster node list and then sorts
// and selects nodes for removal based on the specified strategy. It is
// possible the list does not contain as many nodes as requested. In this case,
// do the limited number available after filtering.
func (si *ScaleIn) identifyTargets(num int, ident *PoolIdentifier, strategy NodeIDStrategy) ([]*api.NodeListStub, error) {

	// Pull a current list of Nomad nodes from the API.
	nodes, _, err := si.nomad.Nodes().List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nomad nodes from API: %v", err)
	}

	// Filter the node list depending on what pool identifier we are using.
	si.log.Debug("filtering node list", "filter", ident.IdentifierKey, "value", ident.Value)

	// Filter our nodes to select only those within our identified pool.
	filteredNodes, err := ident.IdentifyNodes(nodes)
	if err != nil {
		return nil, err
	}

	// TODO(jrasell) this should be removed once the cluster targets and core
	//  autoscaler components are updated to handle reconciliation.
	filteredNodes = filterOutNodeID(filteredNodes, si.curNodeID)

	// Ensure we have not filtered out all the available nodes.
	if len(filteredNodes) == 0 {
		return nil, fmt.Errorf("no nodes unfiltered for %s with value %s", ident.IdentifierKey, ident.Value)
	}

	// Identify the strategy we are using to pick nodes for scale in and
	// perform our list sorting.
	switch strategy {
	case IDStrategyNewestCreateIndex:
	default:
		return nil, fmt.Errorf("unsupported scale in node identification strategy: %q", strategy)
	}

	// If the caller has requested more nodes than we have available once
	// filtered, adjust the value. This shouldn't cause the whole scaling
	// action to fail, but we should warn.
	if num > len(filteredNodes) {
		si.log.Warn("can only identify portion of requested nodes for removal",
			"requested", num, "available", len(filteredNodes))
		num = len(filteredNodes)
	}

	var out []*api.NodeListStub

	// Iterate through our filtered and sorted list of nodes, selecting the
	// required number of nodes to scale in.
	for i := 0; i <= num-1; i++ {
		si.log.Debug("identified Nomad node for removal", "node_id", filteredNodes[i].ID)
		out = append(out, filteredNodes[i])
	}

	return out, nil
}

func (si *ScaleIn) getRemoteIDMap(nodes []*api.NodeListStub, remoteProvider RemoteProvider) ([]NodeID, error) {

	idFunc, ok := idFuncMap[remoteProvider]
	if !ok {
		return nil, fmt.Errorf("remote provider ID function not found: %s", remoteProvider)
	}

	var (
		out  []NodeID
		mErr *multierror.Error
	)

	for _, node := range nodes {

		// Read the full node object from the API which will contain the full
		// information required to identify the remote provider ID.
		nodeInfo, _, err := si.nomad.Nodes().Info(node.ID, nil)
		if err != nil {
			return nil, err
		}

		// Use the identification function to attempt to pull the remote
		// provider ID information from the Node info.
		id, err := idFunc(nodeInfo)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}

		// Add a nice log message for the operators so they can see the node
		// that has been identified if they wish.
		si.log.Debug("identified remote provider ID for node",
			"node_id", nodeInfo.ID, "remote_id", id)

		out = append(out, NodeID{NomadID: node.ID, RemoteID: id})
	}

	return out, mErr.ErrorOrNil()
}

// drainNodes iterates the provided nodeID list and performs a drain on each
// one.
func (si *ScaleIn) drainNodes(ctx context.Context, ignoreSystemJobs bool, deadline time.Duration, nodes []NodeID) error {

	// Define a WaitGroup. This allows us to trigger each node drain in a go
	// routine and then wait for them all to complete before exiting.
	var wg sync.WaitGroup

	// All nodes to be drained form part of a pool of resource and all should
	// use the same DrainSpec.
	drainSpec := api.DrainSpec{Deadline: deadline, IgnoreSystemJobs: ignoreSystemJobs}

	// Define an error to collect errors from each drain routine and a mutex to
	// provide thread safety when calling multierror.Append.
	var (
		result     error
		resultLock sync.Mutex
	)

	for _, node := range nodes {

		// Increment our WaitGroup delta.
		wg.Add(1)

		// Assign our node to a local variable as we are launching a go routine
		// from within a for loop.
		n := node

		// Launch a routine to drain the node. Append any error returned to the
		// error.
		go func() {

			// Ensure we call done on the WaitGroup to decrement the count remaining.
			defer wg.Done()

			if err := si.drainNode(ctx, n.NomadID, &drainSpec); err != nil {
				resultLock.Lock()
				result = multierror.Append(result, err)
				resultLock.Unlock()
			}
		}()
	}

	wg.Wait()

	return result
}

// drainNode triggers a drain on the supplied ID using the DrainSpec. The
// function handles monitoring the drain and reporting its terminal status to
// the caller.
func (si *ScaleIn) drainNode(ctx context.Context, nodeID string, spec *api.DrainSpec) error {

	si.log.Info("triggering drain on node", "node_id", nodeID, "deadline", spec.Deadline)

	// Update the drain on the node.
	resp, err := si.nomad.Nodes().UpdateDrain(nodeID, spec, false, nil)
	if err != nil {
		return fmt.Errorf("failed to drain node: %v", err)
	}

	// Monitor the drain so we output the log messages. An error here indicates
	// the drain failed to complete successfully.
	if err := si.monitorNodeDrain(ctx, nodeID, resp.LastIndex, spec.IgnoreSystemJobs); err != nil {
		return fmt.Errorf("context done while monitoring node drain: %v", err)
	}
	return nil
}

// monitorNodeDrain follows the drain of a node, logging the messages we
// receive to their appropriate level.
func (si *ScaleIn) monitorNodeDrain(ctx context.Context, nodeID string, index uint64, ignoreSys bool) error {
	for msg := range si.nomad.Nodes().MonitorDrain(ctx, nodeID, index, ignoreSys) {
		switch msg.Level {
		case api.MonitorMsgLevelInfo:
			si.log.Info("received node drain message", "node_id", nodeID, "msg", msg.Message)
		case api.MonitorMsgLevelWarn:
			si.log.Warn("received node drain message", "node_id", nodeID, "msg", msg.Message)
		case api.MonitorMsgLevelError:
			return fmt.Errorf("received error while draining node: %s", msg.Message)
		default:
			si.log.Debug("received node drain message", "node_id", nodeID, "msg", msg.Message)
		}
	}
	return ctx.Err()
}

// identifyAutoscalerNodeID identifies the NodeID which the autoscaler is
// running on.
//
// TODO(jrasell) this should be removed once the cluster targets and core
//  autoscaler components are updated to handle reconciliation.
func identifyAutoscalerNodeID(client *api.Client) (string, error) {

	envVar := os.Getenv("NOMAD_ALLOC_ID")
	if envVar == "" {
		return "", nil
	}

	allocInfo, _, err := client.Allocations().Info(envVar, nil)
	if err != nil {
		return "", fmt.Errorf("failed to call Nomad allocations info: %v", err)
	}

	return allocInfo.NodeID, nil
}

// TODO(jrasell) this should be removed once the cluster targets and core
//  autoscaler components are updated to handle reconciliation.
func filterOutNodeID(n []*api.NodeListStub, id string) []*api.NodeListStub {

	if id == "" {
		return n
	}

	var out []*api.NodeListStub

	for _, node := range n {
		if node.ID == id {
			continue
		}
		out = append(out, node)
	}
	return out
}
