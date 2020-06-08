package scaleutils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
)

type ScaleIn struct {
	log   hclog.Logger
	nomad *api.Client
}

// PoolIdentifier is the information used to identify nodes into pools of
// resources. This then forms our scalable unit.
type PoolIdentifier struct {
	IdentifierKey IdentifierKey
	Value         string
}

// NewScaleInUtils returns a new ScaleIn implementation which provides helper
// functions for performing scaling in operations.
func NewScaleInUtils(cfg *api.Config, log hclog.Logger) (*ScaleIn, error) {

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	return &ScaleIn{
		log:   log,
		nomad: client,
	}, nil
}

// RunPreScaleInTasks helps tie together all the tasks required prior to
// scaling in Nomad nodes, and thus terminating the server in the remote
// provider.
func (si *ScaleIn) RunPreScaleInTasks(ctx context.Context, req *ScaleInReq) ([]NodeIDMap, error) {

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

	if err := si.drainNodes(ctx, req.DrainDeadline, nodeIDMap); err != nil {
		return nil, err
	}

	return nodeIDMap, nil
}

// identifyTargets filters the current Nomad cluster node list and then sorts
// and selects nodes for removal based on the specified strategy. It is
// possible the list does not contain any many nodes as requested do to the
// limited number available after filtering.
func (si *ScaleIn) identifyTargets(num int, ident *PoolIdentifier, strategy NodeIDStrategy) ([]*api.NodeListStub, error) {

	// Pull a current list of Nomad nodes from the API.
	nodes, _, err := si.nomad.Nodes().List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nomad nodes from API: %v", err)
	}

	// Filter the node list depending on what pool identifier we are using.
	si.log.Debug("filtering node list", "filter", ident.IdentifierKey, "value", ident.Value)

	switch ident.IdentifierKey {
	case IdentifierKeyClass:
		si.log.Debug("filtering node list by class", "class", ident.Value)
		nodes = filterByClass(nodes, ident.Value)
	default:
		return nil, fmt.Errorf("unsupported node pool identifier: %q", ident.IdentifierKey)
	}

	// Ensure we have not filtered out all the available nodes.
	if len(nodes) == 0 {
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
	if num > len(nodes) {
		si.log.Warn("can only identify portion of requested nodes for removal",
			"requested", num, "available", len(nodes))
		num = len(nodes)
	}

	var out []*api.NodeListStub

	// Iterate through our filtered and sorted list of nodes, selecting the
	// required number of nodes to scale in.
	for i := 0; i <= num-1; i++ {
		si.log.Debug("identified Nomad node for removal", "node_id", nodes[i].ID)
		out = append(out, nodes[i])
	}

	return out, nil
}

func (si *ScaleIn) getRemoteIDMap(nodes []*api.NodeListStub, remoteProvider RemoteProvider) ([]NodeIDMap, error) {

	var idFunc nodeIDMapFunc

	// Depending on the remote provider we are targeting, identify the function
	// we need to use.
	switch remoteProvider {
	case RemoteProviderAWS:
		idFunc = awsNodeIDMap
	default:
		return nil, fmt.Errorf("unsupported remote provider: %q", remoteProvider)
	}

	var (
		out  []NodeIDMap
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

		out = append(out, NodeIDMap{NomadID: node.ID, RemoteID: id})
	}

	return out, mErr.ErrorOrNil()
}

// drainNodes iterates the provides nodeID list and performs a drain on each
// one.
func (si *ScaleIn) drainNodes(ctx context.Context, deadline time.Duration, nodes []NodeIDMap) error {

	// Define a WaitGroup. This allows us to trigger each node drain in a go
	// routine and then wait for them all to complete before exiting.
	var wg sync.WaitGroup

	// All nodes to be drained form part of a pool of resource and all should
	// use the same DrainSpec.
	drainSpec := api.DrainSpec{Deadline: deadline}

	// Define an error to collect errors from each drain routine.
	var result error

	for _, node := range nodes {

		// Increment our WaitGroup delta.
		wg.Add(1)

		// Assign our node to a local variable as we are launching a go routine
		// from within a for loop.
		n := node

		// Launch a routine to drain the node. Append any error returned to the
		// error.
		go func() {
			if err := si.drainNode(ctx, &wg, n.NomadID, &drainSpec); err != nil {
				result = multierror.Append(result, err)
			}
		}()
	}

	wg.Wait()

	return result
}

// drainNode triggers a drain on the supplied ID using the DrainSpec. The
// function handles monitoring the drain and reporting its terminal status to
// the caller.
func (si *ScaleIn) drainNode(ctx context.Context, wg *sync.WaitGroup, nodeID string, spec *api.DrainSpec) error {

	si.log.Info("triggering drain on node", "node_id", nodeID, "deadline", spec.Deadline)

	// Ensure we call done on the WaitGroup to decrement the count remaining.
	defer wg.Done()

	// Update the drain on the node.
	resp, err := si.nomad.Nodes().UpdateDrain(nodeID, spec, false, nil)
	if err != nil {
		return fmt.Errorf("failed to drain node: %v", err)
	}

	// Monitor the drain so we output the log messages. An error here indicates
	// the drain failed to complete successfully.
	if err := si.monitorNodeDrain(ctx, nodeID, resp.LastIndex); err != nil {
		return fmt.Errorf("context done while monitoring node drain: %v", err)
	}
	return nil
}

// monitorNodeDrain follows the drain of a node, logging the messages we
// receive to their appropriate level.
//
// TODO(jrasell): currently the ignoreSys param is hardcoded to false, we will
//  probably want to expose this to operators in the future.
func (si *ScaleIn) monitorNodeDrain(ctx context.Context, nodeID string, index uint64) error {
	for msg := range si.nomad.Nodes().MonitorDrain(ctx, nodeID, index, false) {
		switch msg.Level {
		case api.MonitorMsgLevelInfo:
			si.log.Info("received node drain message", "node_id", nodeID, "msg", msg)
		case api.MonitorMsgLevelWarn:
			si.log.Warn("received node drain message", "node_id", nodeID, "msg", msg)
		case api.MonitorMsgLevelError:
			return fmt.Errorf("received error while draining node: %s", msg.Message)
		default:
			si.log.Debug("received node drain message", "node_id", nodeID, "msg", msg)
		}
	}
	return ctx.Err()
}
