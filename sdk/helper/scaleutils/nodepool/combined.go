// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nodepool

import "github.com/hashicorp/nomad/api"

// ClusterNodePoolIdentifierList is a list of ClusterNodePoolIdentifier
// values that filters nodes using AND logic. A node must match every
// identifier in the list to be considered a pool member.
type ClusterNodePoolIdentifierList []ClusterNodePoolIdentifier

// IsPoolMember returns true if the node matches all identifiers in the list.
func (l ClusterNodePoolIdentifierList) IsPoolMember(n *api.NodeListStub) bool {
	for _, id := range l {
		if !id.IsPoolMember(n) {
			return false
		}
	}
	return true
}
