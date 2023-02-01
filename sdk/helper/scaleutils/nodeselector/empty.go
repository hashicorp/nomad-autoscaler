// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	client           *api.Client
	log              hclog.Logger
	ignoreSystemJobs bool
}

// newEmptyClusterScaleInNodeSelector returns a new
// emptyClusterScaleInNodeSelector implementation of the
// ClusterScaleInNodeSelector interface.
func newEmptyClusterScaleInNodeSelector(c *api.Client, log hclog.Logger, ignoreSystemJobs bool) ClusterScaleInNodeSelector {
	return &emptyClusterScaleInNodeSelector{
		client:           c,
		log:              log,
		ignoreSystemJobs: ignoreSystemJobs,
	}
}

// Name satisfies the Name function on the ClusterScaleInNodeSelector
// interface.
func (e *emptyClusterScaleInNodeSelector) Name() string {
	if e.ignoreSystemJobs {
		return sdk.TargetNodeSelectorStrategyEmptyIgnoreSystemJobs
	}
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
		if e.shouldIgnoreAlloc(alloc) {
			continue
		}

		e.log.Debug("selector skipped node with non-terminal allocs", "node_id", id)
		return true
	}
	return false
}

func (e *emptyClusterScaleInNodeSelector) shouldIgnoreAlloc(alloc *api.Allocation) bool {
	ignore := false

	// Ignore alloc if it's in a terminal state.
	ignore = ignore || (alloc.ClientTerminalStatus() || alloc.ServerTerminalStatus())

	// Ignore system jobs if requested.
	if e.ignoreSystemJobs {
		ignore = ignore || *alloc.Job.Type == api.JobTypeSystem
	}

	return ignore
}
