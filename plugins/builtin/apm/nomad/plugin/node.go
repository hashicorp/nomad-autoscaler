// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
	"github.com/hashicorp/nomad/api"
)

// nodePoolQuery is the plugins internal representation of a query and contains
// all the information needed to perform a Nomad APM query for a node pool.
type nodePoolQuery struct {
	metric         string
	poolIdentifier nodepool.ClusterNodePoolIdentifier
	operation      string
}

type nodePoolResources struct {
	allocatable *poolResources
	allocated   *poolResources
}

type poolResources struct {
	cpu int64
	mem int64
}

// queryNodePool is the main entry point when performing a Nomad node pool APM
// query.
func (a *APMPlugin) queryNodePool(q string) (sdk.TimestampedMetrics, error) {

	// Parse our query to ensure we have all the information required to
	// continue.
	query, err := parseNodePoolQuery(q)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %v", err)
	}
	a.logger.Debug("performing node pool APM query", "query", q)

	// Identify the resource available and consumed within the target pool.
	resources, err := a.getPoolResources(query.poolIdentifier)
	if err != nil {
		return nil, err
	}
	a.logger.Debug("collected node pool resource data",
		"allocated_cpu", resources.allocated.cpu, "allocated_memory", resources.allocated.mem,
		"allocatable_cpu", resources.allocatable.cpu, "allocatable_memory", resources.allocatable.mem)

	var result float64

	// There is no need for a default catch all here as the metric has been
	// validated during the query parsing.
	switch query.metric {
	case queryMetricMem:
		if resources.allocatable.mem == 0 {
			return nil, errors.New("zero allocatable memory found in pool")
		}
		result = calculateNodePoolResult(float64(resources.allocated.mem), float64(resources.allocatable.mem))
	case queryMetricCPU:
		if resources.allocatable.cpu == 0 {
			return nil, errors.New("zero allocatable cpu found in pool")
		}
		result = calculateNodePoolResult(float64(resources.allocated.cpu), float64(resources.allocatable.cpu))
	}

	tm := sdk.TimestampedMetric{
		Timestamp: time.Now(),
		Value:     result,
	}
	return sdk.TimestampedMetrics{tm}, nil
}

// getPoolResources gathers the allocatable and allocated resources for the
// specified node pool. Any error in calling the Nomad API for details will
// result in an error. This is because with missing data, we cannot reliably
// make calculations.
func (a *APMPlugin) getPoolResources(id nodepool.ClusterNodePoolIdentifier) (*nodePoolResources, error) {

	client, config := a.getClientAndConfigSnapshot()
	if client == nil {
		return nil, errors.New("nomad client not configured")
	}

	nodes, _, err := client.Nodes().List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nomad nodes: %v", err)
	}

	filterOpts, err := scaleutils.NewNodeFilterOptions(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse node filter options: %v", err)
	}

	// Perform our node filtering so we are left with a list of nodes that form
	// our pool and that are in the correct state.
	nodePoolList, err := scaleutils.FilterNodesWithOptions(nodes, id.IsPoolMember, filterOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to identify nodes within pool: %v", err)
	}
	if len(nodePoolList) == 0 {
		return nil, errors.New("no nodes identified within pool")
	}

	// Ensure we instantiate the whole object.
	resp := nodePoolResources{
		allocatable: &poolResources{},
		allocated:   &poolResources{},
	}

	// Loop here so we only need to iterate the list of nodes once.
	for _, node := range nodePoolList {

		// If we get a single error when performing the following lookups we
		// cannot reliably make calculations.
		if err := a.getNodeAllocatableResources(client, node.ID, resp.allocatable); err != nil {
			return nil, fmt.Errorf("failed to get allocatable resources on node %s: %v", node.ID, err)
		}
		if err := a.getNodeAllocatedResources(client, node.ID, resp.allocated); err != nil {
			return nil, fmt.Errorf("failed to get allocated resources on node %s: %v", node.ID, err)
		}
	}

	return &resp, nil
}

// getNodeAllocatableResources updates the poolResources tracking with the
// allocatable resources on the node.
func (a *APMPlugin) getNodeAllocatableResources(client *api.Client, nodeID string, pool *poolResources) error {

	nodeInfo, _, err := client.Nodes().Info(nodeID, nil)
	if err != nil {
		return fmt.Errorf("failed to read Nomad node info: %v", err)
	}
	if nodeInfo == nil || nodeInfo.NodeResources == nil || nodeInfo.ReservedResources == nil {
		return errors.New("node resources are incomplete")
	}

	// Update our tracking, making sure to account for reserved resources
	// on the node.
	pool.cpu += nodeInfo.NodeResources.Cpu.CpuShares - int64(nodeInfo.ReservedResources.Cpu.CpuShares)
	pool.mem += nodeInfo.NodeResources.Memory.MemoryMB - int64(nodeInfo.ReservedResources.Memory.MemoryMB)

	return nil
}

// getNodeAllocatedResources updates the poolResources tracking with the
// allocated resources on the node.
func (a *APMPlugin) getNodeAllocatedResources(client *api.Client, nodeID string, pool *poolResources) error {

	nodeAllocs, _, err := client.Nodes().Allocations(nodeID, nil)
	if err != nil {
		return fmt.Errorf("failed to read Nomad node allocs : %v", err)
	}

	for _, alloc := range nodeAllocs {

		// Do not use the allocations stats if it is in a terminal status.
		if isServerTerminalStatus(alloc) || isClientTerminalStatus(alloc) {
			continue
		}
		if alloc.Resources == nil || alloc.Resources.CPU == nil || alloc.Resources.MemoryMB == nil {
			return errors.New("allocation resources are incomplete")
		}

		// Update our tracking with the resources of the allocation.
		pool.cpu += int64(*alloc.Resources.CPU)
		pool.mem += int64(*alloc.Resources.MemoryMB)
	}

	return nil
}

func parseNodePoolQuery(q string) (*nodePoolQuery, error) {

	// Try splitting into 3 parts first (old format: query/value/key).
	// Then check for 2-part combined format (query/key1=val1+key2=val2).
	mainParts := strings.SplitN(q, "/", 3)

	var query nodePoolQuery

	switch {
	// New combined format: node_<op>_<metric>/key1=val1+key2=val2
	// Detected by: exactly 2 parts, and the second part contains "="
	case len(mainParts) >= 2 && strings.Contains(mainParts[1], "=") && (len(mainParts) == 2 || (len(mainParts) == 3 && mainParts[2] == "")):
		ids, err := nodepool.DecodeCombinedQueryIdentifiers(mainParts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse combined pool identifiers: %v", err)
		}
		if len(ids) == 1 {
			query.poolIdentifier = ids[0]
		} else {
			query.poolIdentifier = nodepool.NewCombinedClusterPoolIdentifier(ids, nodepool.CombinedClusterPoolIdentifierAnd)
		}

	// Old format: node_<op>_<metric>/<value>/<key>
	// Detected by: 3 parts where the third part is a recognized pool key.
	case len(mainParts) == 3 && isValidPoolKey(mainParts[2]):
		// Guard against ambiguous queries where the value looks like a
		// combined key=value pair (e.g., "node_class=x" as a value with
		// "datacenter" as the key). This is almost certainly a user mistake.
		if strings.Contains(mainParts[1], "=") {
			return nil, fmt.Errorf("ambiguous query: value %q looks like combined format but key %q was specified; use combined format instead: <query>/key1=val1+key2=val2", mainParts[1], mainParts[2])
		}
		switch mainParts[2] {
		case sdk.TargetConfigKeyClass, "class":
			query.poolIdentifier = nodepool.NewNodeClassPoolIdentifier(mainParts[1])
		case sdk.TargetConfigKeyDatacenter:
			query.poolIdentifier = nodepool.NewNodeDatacenterPoolIdentifier(mainParts[1])
		case sdk.TargetConfigKeyNodePool:
			query.poolIdentifier = nodepool.NewNodePoolClusterPoolIdentifier(mainParts[1])
		}

	default:
		return nil, fmt.Errorf("expected <query>/<pool_identifier_value>/<pool_identifier_key> or <query>/<key>=<value>[+<key>=<value>...], received %s", q)
	}

	opMetricParts := strings.SplitN(mainParts[0], "_", 3)
	if len(opMetricParts) != 3 {
		return nil, fmt.Errorf("expected node_<operation>_<metric>, received %s", mainParts[0])
	}

	if err := validateMetricNodeQuery(opMetricParts[2]); err != nil {
		return nil, err
	}
	query.metric = opMetricParts[2]

	switch opMetricParts[1] {
	case queryOpPercentageAllocated:
		query.operation = opMetricParts[1]
	default:
		return nil, fmt.Errorf("invalid operation %q, allowed value is %s",
			opMetricParts[1], queryOpPercentageAllocated)
	}
	return &query, nil
}

// isValidPoolKey returns true if key is a recognized pool identifier key
// in the old 3-part query format.
func isValidPoolKey(key string) bool {
	switch key {
	case sdk.TargetConfigKeyClass, "class", sdk.TargetConfigKeyDatacenter, sdk.TargetConfigKeyNodePool:
		return true
	default:
		return false
	}
}

func validateMetricNodeQuery(metric string) error {
	return validateMetric(metric, []string{queryMetricCPU, queryMetricMem})
}

// calculateNodePoolResult returns the current usage percentage of the node
// pool.
func calculateNodePoolResult(allocated, allocatable float64) float64 {
	return allocated * 100 / allocatable
}

// isServerTerminalStatus returns true if the desired state of the allocation
// is terminal.
func isServerTerminalStatus(alloc *api.Allocation) bool {
	switch alloc.DesiredStatus {
	case api.AllocDesiredStatusRun:
		return false
	default:
		return true
	}
}

// isClientTerminalStatus returns true if the desired state of the allocation
// is terminal.
func isClientTerminalStatus(alloc *api.Allocation) bool {
	switch alloc.ClientStatus {
	case api.AllocClientStatusComplete, api.AllocClientStatusFailed, api.AllocClientStatusLost:
		return true
	default:
		return false
	}
}
