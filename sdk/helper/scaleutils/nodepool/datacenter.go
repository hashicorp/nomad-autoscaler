// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

type nodeDatacenterClusterPoolIdentifier struct {
	id                  string
	ignoreDrainingNodes bool
}

// NewNodeDatacenterPoolIdentifier returns a new
// nodeDatacenterClusterPoolIdentifier implementation of the
// ClusterNodePoolIdentifier interface.
func NewNodeDatacenterPoolIdentifier(id string, ignoreDrainingNodes bool) ClusterNodePoolIdentifier {
	return &nodeDatacenterClusterPoolIdentifier{
		id:                  id,
		ignoreDrainingNodes: ignoreDrainingNodes,
	}
}

// IsPoolMember satisfies the IsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (n nodeDatacenterClusterPoolIdentifier) IsPoolMember(node *api.NodeListStub) bool {
	if n.ignoreDrainingNodes {
		return node.Datacenter == n.id && !node.Drain
	}
	return node.Datacenter == n.id
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (n nodeDatacenterClusterPoolIdentifier) Key() string { return sdk.TargetConfigKeyDatacenter }

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (n nodeDatacenterClusterPoolIdentifier) Value() string { return n.id }
