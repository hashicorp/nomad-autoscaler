// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// nodePoolClusterPoolIdentifier is the NodePool implementation of the
// ClusterNodePoolIdentifier interface that filters Nomad nodes by their
// Node.NodePool parameter.
type nodePoolClusterPoolIdentifier struct {
	pool string
}

// NewNodePoolClusterPoolIdentifier returns a new nodePoolClusterPoolIdentifier
// implementation of the ClusterNodePoolIdentifier interface.
func NewNodePoolClusterPoolIdentifier(pool string) ClusterNodePoolIdentifier {
	return &nodePoolClusterPoolIdentifier{
		pool: pool,
	}
}

// IsPoolMember satisfies the IsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (n nodePoolClusterPoolIdentifier) IsPoolMember(node *api.NodeListStub) bool {
	return node.NodePool == n.pool
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (n nodePoolClusterPoolIdentifier) Key() string { return sdk.TargetConfigKeyNodePool }

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (n nodePoolClusterPoolIdentifier) Value() string { return n.pool }
