// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"fmt"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// ClusterNodePoolIdentifier is the interface that defines how nodes are
// classed into pools of resources.
type ClusterNodePoolIdentifier interface {

	// IsPoolMember identifies whether the node is a member of the node pool.
	// It is the responsibility of the implementation to read the node info if
	// the required information is not within the stub struct.
	IsPoolMember(*api.NodeListStub) bool

	// Key returns the string representation of the pool identifier.
	Key() string

	// Value returns the pool identifier value that nodes are being filtered
	// by.
	Value() string
}

// NewClusterNodePoolIdentifierList generates a ClusterNodePoolIdentifierList
// from the provided configuration. It extracts all recognized pool-identifying
// keys (node_class, datacenter, node_pool) and returns a list that filters
// nodes using AND logic. An error is returned if no valid keys are found.
func NewClusterNodePoolIdentifierList(cfg map[string]string) (ClusterNodePoolIdentifierList, error) {
	class, hasClass := cfg[sdk.TargetConfigKeyClass]
	dc, hasDC := cfg[sdk.TargetConfigKeyDatacenter]
	pool, hasPool := cfg[sdk.TargetConfigKeyNodePool]

	ids := make(ClusterNodePoolIdentifierList, 0, 1)
	if hasClass {
		ids = append(ids, NewNodeClassPoolIdentifier(class))
	}
	if hasDC {
		ids = append(ids, NewNodeDatacenterPoolIdentifier(dc))
	}
	if hasPool {
		ids = append(ids, NewNodePoolClusterPoolIdentifier(pool))
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("node pool identification method required")
	}
	return ids, nil
}

// Deprecated: Use NewClusterNodePoolIdentifierList for multi-key AND filtering.
// NewClusterNodePoolIdentifier generates a new ClusterNodePoolIdentifier based
// on the provided configuration. If a valid option is not found, an error will
// be returned.
// TODO: Remove this function in next Release.
func NewClusterNodePoolIdentifier(cfg map[string]string) (ClusterNodePoolIdentifier, error) {
	class, hasClass := cfg[sdk.TargetConfigKeyClass]
	dc, hasDC := cfg[sdk.TargetConfigKeyDatacenter]
	pool, hasPool := cfg[sdk.TargetConfigKeyNodePool]

	ids := make([]ClusterNodePoolIdentifier, 0, 1)
	if hasClass {
		ids = append(ids, NewNodeClassPoolIdentifier(class))
	}
	if hasDC {
		ids = append(ids, NewNodeDatacenterPoolIdentifier(dc))
	}
	if hasPool {
		ids = append(ids, NewNodePoolClusterPoolIdentifier(pool))
	}

	switch len(ids) {
	case 0:
		return nil, fmt.Errorf("node pool identification method required")
	case 1:
		return ids[0], nil
	default:
		return NewCombinedClusterPoolIdentifier(ids, CombinedClusterPoolIdentifierAnd), nil
	}
}
