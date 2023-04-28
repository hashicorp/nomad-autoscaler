// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"math"
	"sort"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// leastBusyClusterScaleInNodeSelector is the NodeSelector implementation of
// the ClusterScaleInNodeSelector interface. It selects nodes for termination
// based on their total resource allocation; choosing those with the lowest
// value.
type leastBusyClusterScaleInNodeSelector struct {
	client *api.Client
	log    hclog.Logger
}

// newLeastBusyClusterScaleInNodeSelector returns a new
// leastBusyClusterScaleInNodeSelector implementation of the
// ClusterScaleInNodeSelector interface.
func newLeastBusyClusterScaleInNodeSelector(c *api.Client, log hclog.Logger) ClusterScaleInNodeSelector {
	return &leastBusyClusterScaleInNodeSelector{
		client: c,
		log:    log,
	}
}

// Name satisfies the Name function on the ClusterScaleInNodeSelector
// interface.
func (l *leastBusyClusterScaleInNodeSelector) Name() string {
	return sdk.TargetNodeSelectorStrategyLeastBusy
}

// Select satisfies the Select function on the ClusterScaleInNodeSelector
// interface.
func (l *leastBusyClusterScaleInNodeSelector) Select(nodes []*api.NodeListStub, num int) []*api.NodeListStub {

	if len(nodes) < num {
		num = len(nodes)
	}

	// Setup the array. It must be the length of the input node list, as we
	// must iterate the whole list to correctly select nodes.
	h := make(nodeResourceConsumption, len(nodes))

	for i, node := range nodes {

		// The NodeResources object is only available within the Nomad
		// api.NodeListStub from v1.0.0 onwards. Therefore in clusters running
		// clients on lower versions, we need additional API calls to discover
		// this information.
		if node.NodeResources == nil {

			nodeInfo, _, err := l.client.Nodes().Info(node.ID, nil)
			if err != nil {
				l.log.Error("failed to call node info for resource detail", "error", err)
				return nil
			}
			if nodeInfo.NodeResources == nil {
				l.log.Error("node object does not contain resource info", "node_id", node.ID)
				return nil
			}

			// Update the node to include the resource object.
			node.NodeResources = nodeInfo.NodeResources

			if node.ReservedResources == nil && nodeInfo.ReservedResources != nil {
				node.ReservedResources = nodeInfo.ReservedResources
			}
		}

		// In the event of an API error, we are unable to ensure the selected
		// nodes are the least busy. Therefore we must exit. In the future we
		// could consider adding a threshold, configurable by the operator to
		// allow for network hiccups.
		resource, err := l.computeNodeResources(node)
		if err != nil {
			l.log.Error("failed to calculate node resource usage", "error", err)
			return nil
		}
		h[i] = resource
	}

	// Perform the sorting; do not forget this part.
	sort.Sort(h)

	out := make([]*api.NodeListStub, num)

	// Remove entries off our array, using the api.NodeListStub to quickly add
	// the required object to our return object.
	for i := 0; i <= num-1; i++ {
		out[i] = h[i].node
	}
	return out
}

// computeNodeResources calculates the node CPU and Memory resource allocation
// for entry into the array. Any error here is terminal and means we are unable
// to safely understand the least allocated nodes.
func (l *leastBusyClusterScaleInNodeSelector) computeNodeResources(node *api.NodeListStub) (*nodeResourceStats, error) {

	nodeAllocs, _, err := l.client.Nodes().Allocations(node.ID, nil)
	if err != nil {
		return nil, err
	}

	allocated := computeNodeAllocatedResources(nodeAllocs)
	total := computeNodeTotalResources(node)

	mem := computeNodeAllocatedPercentage(allocated.Memory.MemoryMB, total.Memory.MemoryMB)
	cpu := computeNodeAllocatedPercentage(allocated.Cpu.CpuShares, total.Cpu.CpuShares)

	return &nodeResourceStats{
		consumed: math.Ceil((mem + cpu) / 2),
		node:     node,
	}, nil
}

// computeNodeAllocatedResources calculates the amount of allocated resource on
// the node as used by running allocations.
func computeNodeAllocatedResources(allocs []*api.Allocation) *api.NodeResources {
	var mem, cpu int

	for _, alloc := range allocs {
		if alloc.ClientStatus != api.AllocClientStatusRunning {
			continue
		}
		mem += *alloc.Resources.MemoryMB
		cpu += *alloc.Resources.CPU
	}

	return &api.NodeResources{
		Cpu:    api.NodeCpuResources{CpuShares: int64(cpu)},
		Memory: api.NodeMemoryResources{MemoryMB: int64(mem)},
	}
}

// computeNodeTotalResources calculates the node allocatable resources and
// takes into account reserved resources.
func computeNodeTotalResources(node *api.NodeListStub) *api.NodeResources {
	total := api.NodeResources{}

	r := node.NodeResources
	res := node.ReservedResources
	if res == nil {
		res = &api.NodeReservedResources{
			Cpu:    api.NodeReservedCpuResources{},
			Memory: api.NodeReservedMemoryResources{},
		}
	}
	total.Cpu.CpuShares = r.Cpu.CpuShares - int64(res.Cpu.CpuShares)
	total.Memory.MemoryMB = r.Memory.MemoryMB - int64(res.Memory.MemoryMB)
	return &total
}

// computeNodeAllocatedPercentage helps calculate the percentage allocation of
// a node resource.
func computeNodeAllocatedPercentage(allocated, total int64) float64 {
	return math.Ceil((float64(allocated) * float64(100)) / float64(total))
}

// nodeResourceStats encapsulates a single nodes resource consumption so that
// we can correctly perform our calculations to find the least allocated nodes
// for termination.
type nodeResourceStats struct {

	// consumed is the total percentage of memory and CPU that is consumed on
	// the node. This provides the basis for node comparison.
	consumed float64

	// node is a pointer to the node which this entry represents. It provides
	// an easy way to pop entries in the way we need to return a list to the
	// caller.
	node *api.NodeListStub
}

// nodeResourceConsumption is our sort.Interface implementation.
type nodeResourceConsumption []*nodeResourceStats

// Len satisfies the Len function on the sort.Interface interface.
func (nrc nodeResourceConsumption) Len() int { return len(nrc) }

// Less satisfies the Less function on the sort.Interface interface.
func (nrc nodeResourceConsumption) Less(i, j int) bool {
	return nrc[i].consumed < nrc[j].consumed
}

// Swap satisfies the Swap function on the sort.Interface interface.
func (nrc nodeResourceConsumption) Swap(i, j int) {
	nrc[i], nrc[j] = nrc[j], nrc[i]
}
