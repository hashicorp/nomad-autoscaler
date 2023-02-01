// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

type nodeDatacenterClusterPoolIdentifier struct {
	id string
}

// NewNodeDatacenterPoolIdentifier returns a new
// nodeDatacenterClusterPoolIdentifier implementation of the
// ClusterNodePoolIdentifier interface.
func NewNodeDatacenterPoolIdentifier(id string) ClusterNodePoolIdentifier {
	return &nodeDatacenterClusterPoolIdentifier{
		id: id,
	}
}

// IsPoolMember satisfies the IsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (n nodeDatacenterClusterPoolIdentifier) IsPoolMember(node *api.NodeListStub) bool {
	return node.Datacenter == n.id
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (n nodeDatacenterClusterPoolIdentifier) Key() string { return sdk.TargetConfigKeyDatacenter }

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (n nodeDatacenterClusterPoolIdentifier) Value() string { return n.id }
