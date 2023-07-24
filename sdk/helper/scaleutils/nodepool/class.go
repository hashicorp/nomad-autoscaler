// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// defaultClassIdentifier is the class value used when the nodes NodeClass is
// empty.
const defaultClassIdentifier = "autoscaler-default-pool"

// nodeClassClusterPoolIdentifier is the NodeClass implementation of the
// ClusterNodePoolIdentifier interface and filters Nomad nodes by their
// Node.NodeClass parameter.
type nodeClassClusterPoolIdentifier struct {
	id                  string
	ignoreDrainingNodes bool
}

// NewNodeClassPoolIdentifier returns a new nodeClassClusterPoolIdentifier
// implementation of the ClusterNodePoolIdentifier interface.
func NewNodeClassPoolIdentifier(id string, ignoreDrainingNodes bool) ClusterNodePoolIdentifier {
	return &nodeClassClusterPoolIdentifier{
		id:                  id,
		ignoreDrainingNodes: ignoreDrainingNodes,
	}
}

// IsPoolMember satisfies the IsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (n nodeClassClusterPoolIdentifier) IsPoolMember(node *api.NodeListStub) bool {
	isNodeClassMatch := node.NodeClass != "" && node.NodeClass == n.id
	isNodeDefault := node.NodeClass == "" && n.id == defaultClassIdentifier
	if n.ignoreDrainingNodes {
		return (isNodeClassMatch || isNodeDefault) && !node.Drain
	}
	return isNodeClassMatch || isNodeDefault
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (n nodeClassClusterPoolIdentifier) Key() string { return sdk.TargetConfigKeyClass }

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (n nodeClassClusterPoolIdentifier) Value() string { return n.id }
