// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestClusterNodePoolIdentifierList_IsPoolMember(t *testing.T) {
	testCases := []struct {
		name     string
		ids      ClusterNodePoolIdentifierList
		nodes    []*api.NodeListStub
		expected []bool
	}{
		{
			name: "single class identifier",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("test-class"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "test-class", Datacenter: "dc1"},
				{NodeClass: "other-class", Datacenter: "dc1"},
			},
			expected: []bool{true, false},
		},
		{
			name: "single datacenter identifier",
			ids: ClusterNodePoolIdentifierList{
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "test-class", Datacenter: "dc1"},
				{NodeClass: "test-class", Datacenter: "dc2"},
			},
			expected: []bool{true, false},
		},
		{
			name: "combined class and datacenter (AND logic)",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("test-class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "test-class", Datacenter: "dc1"},
				{NodeClass: "test-class", Datacenter: "dc2"},
				{NodeClass: "other-class", Datacenter: "dc1"},
				{NodeClass: "other-class", Datacenter: "dc2"},
			},
			expected: []bool{true, false, false, false},
		},
		{
			name: "combined class, datacenter, and node_pool (AND logic)",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("test-class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
				NewNodePoolClusterPoolIdentifier("gpu"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "test-class", Datacenter: "dc1", NodePool: "gpu"},
				{NodeClass: "test-class", Datacenter: "dc1", NodePool: "default"},
				{NodeClass: "test-class", Datacenter: "dc2", NodePool: "gpu"},
			},
			expected: []bool{true, false, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, node := range tc.nodes {
				got := tc.ids.IsPoolMember(node)
				assert.Equal(t, tc.expected[i], got)
			}
		})
	}
}
