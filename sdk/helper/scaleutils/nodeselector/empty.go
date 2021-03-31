package nodeselector

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// emptyClusterScaleInNodeSelector is the NodeSelector implementation of the
// ClusterScaleInNodeSelector interface. It selects nodes for termination only
// if they do not have any non-terminal allocations, and therefore can be
// classed as empty.
type emptyClusterScaleInNodeSelector struct {
	client *api.Client
	log    hclog.Logger
}

// newEmptyClusterScaleInNodeSelector returns a new
// emptyClusterScaleInNodeSelector implementation of the
// ClusterScaleInNodeSelector interface.
func newEmptyClusterScaleInNodeSelector(c *api.Client, log hclog.Logger) ClusterScaleInNodeSelector {
	return &emptyClusterScaleInNodeSelector{
		client: c,
		log:    log,
	}
}

// Name satisfies the Name function on the ClusterScaleInNodeSelector
// interface.
func (e *emptyClusterScaleInNodeSelector) Name() string {
	return sdk.TargetNodeSelectorStrategyEmpty
}

// Select satisfies the Select function on the ClusterScaleInNodeSelector
// interface.
func (e *emptyClusterScaleInNodeSelector) Select(nodes []*api.NodeListStub, num int) []*api.NodeListStub {

	if len(nodes) < num {
		num = len(nodes)
	}
	var out []*api.NodeListStub

	for _, node := range nodes {

		// If the node has non-terminal allocations, this isn't the node we are
		// looking for. Otherwise add this node to our selected list.
		if e.nodeHasAllocs(node.ID) {
			continue
		}
		out = append(out, node)

		// Short circuit if we have found a required number of nodes. We do not
		// need to do any further filtering.
		if len(out) == num {
			break
		}
	}

	return out
}

// nodeHasAllocs returns whether the node has non-terminal allocations.
func (e *emptyClusterScaleInNodeSelector) nodeHasAllocs(id string) bool {

	allocs, _, err := e.client.Nodes().Allocations(id, nil)
	if err != nil {
		e.log.Error("failed to detail node allocs", "error", err)
		return true
	}

	for _, alloc := range allocs {
		if alloc.ClientTerminalStatus() || alloc.ServerTerminalStatus() {
			e.log.Debug("selector skipped node with non-terminal allocs", "node_id", id)
			return true
		}
	}
	return false
}
