// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestClusterNodePoolIdentifierList_IsPoolMember(t *testing.T) {
	testCases := []struct {
		name     string
		ids      ClusterNodePoolIdentifierList
		nodes    []*api.NodeListStub
		expected []bool
	}{
		{
			name: "find node by class and datacenter",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("test-class"),
				NewNodeDatacenterPoolIdentifier("test-dc"),
			},
			nodes: []*api.NodeListStub{
				{
					NodeClass:  "test-class",
					Datacenter: "test-dc",
				},
				{
					NodeClass: "test-class",
				},
				{
					Datacenter: "test-dc",
				},
				{
					NodeClass:  "test-wrong-class",
					Datacenter: "test-dc",
				},
				{
					NodeClass:  "test-wrong-class",
					Datacenter: "test-wrong-dc",
				},
			},
			expected: []bool{true, false, false, false, false},
		},
		{
			name: "single identifier",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("gpu"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "gpu"},
				{NodeClass: "cpu"},
			},
			expected: []bool{true, false},
		},
		{
			name: "all three identifiers",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("gpu"),
				NewNodeDatacenterPoolIdentifier("dc1"),
				NewNodePoolClusterPoolIdentifier("prod"),
			},
			nodes: []*api.NodeListStub{
				{NodeClass: "gpu", Datacenter: "dc1", NodePool: "prod"},
				{NodeClass: "gpu", Datacenter: "dc1", NodePool: "dev"},
				{NodeClass: "gpu", Datacenter: "dc2", NodePool: "prod"},
			},
			expected: []bool{true, false, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, node := range tc.nodes {
				got := tc.ids.IsPoolMember(node)
				must.Eq(t, tc.expected[i], got)
			}
		})
	}
}

func TestClusterNodePoolIdentifierList_Encode(t *testing.T) {
	testCases := []struct {
		name     string
		ids      ClusterNodePoolIdentifierList
		expected string
	}{
		{
			name: "single identifier",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("gpu"),
			},
			expected: "node_class=gpu",
		},
		{
			name: "two identifiers",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("hashistack"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			expected: "node_class=hashistack+datacenter=dc1",
		},
		{
			name: "three identifiers",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("gpu"),
				NewNodeDatacenterPoolIdentifier("us-east-1a"),
				NewNodePoolClusterPoolIdentifier("default"),
			},
			expected: "node_class=gpu+datacenter=us-east-1a+node_pool=default",
		},
		{
			name: "value with special characters",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("special+class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			expected: "node_class=special%2Bclass+datacenter=dc1",
		},
		{
			name: "value with equals sign",
			ids: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("key=value"),
			},
			expected: "node_class=key%3Dvalue",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.ids.Encode()
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestDecodeCombinedQueryIdentifiers(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedIDs ClusterNodePoolIdentifierList
		expectError bool
		errorMsg    string
	}{
		{
			name:  "single node_class",
			input: "node_class=gpu",
			expectedIDs: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("gpu"),
			},
		},
		{
			name:  "node_class and datacenter",
			input: "node_class=hashistack+datacenter=dc1",
			expectedIDs: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("hashistack"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
		},
		{
			name:  "URL-encoded value with plus",
			input: "node_class=special%2Bclass+datacenter=dc1",
			expectedIDs: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("special+class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
		},
		{
			name:  "URL-encoded value with equals",
			input: "node_class=key%3Dvalue",
			expectedIDs: ClusterNodePoolIdentifierList{
				NewNodeClassPoolIdentifier("key=value"),
			},
		},
		{
			name:        "legacy class key is now rejected",
			input:       "class=hashistack+datacenter=dc1",
			expectError: true,
			errorMsg:    "unknown pool key \"class\"",
		},
		{
			name:        "invalid key",
			input:       "invalid_key=value",
			expectError: true,
			errorMsg:    "unknown pool key \"invalid_key\"",
		},
		{
			name:        "empty value",
			input:       "node_class=",
			expectError: true,
			errorMsg:    "invalid pool identifier",
		},
		{
			name:        "missing equals",
			input:       "node_class",
			expectError: true,
			errorMsg:    "invalid pool identifier",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := DecodeCombinedQueryIdentifiers(tc.input)
			if tc.expectError {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.errorMsg)
				must.Nil(t, ids)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expectedIDs, ids)
			}
		})
	}
}
