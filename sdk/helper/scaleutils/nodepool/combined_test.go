// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

type mockClusterPoolIdentifier struct{}

func NewMockPoolIdentifier() ClusterNodePoolIdentifier                     { return &mockClusterPoolIdentifier{} }
func (m *mockClusterPoolIdentifier) IsPoolMember(n *api.NodeListStub) bool { return true }
func (m *mockClusterPoolIdentifier) Key() string                           { return "mock_identifier" }
func (c *mockClusterPoolIdentifier) Value() string                         { return "mock_value" }

func TestNewCombinedPoolIdentifier(t *testing.T) {
	testCases := []struct {
		name          string
		identifier    ClusterNodePoolIdentifier
		expectedValue string
	}{
		{
			name: "single identifier",
			identifier: NewCombinedClusterPoolIdentifier(
				[]ClusterNodePoolIdentifier{
					NewNodeClassPoolIdentifier("test-class"),
				},
				CombinedClusterPoolIdentifierAnd,
			),
			expectedValue: "node_class:test-class",
		},
		{
			name: "multiple identifiers with and",
			identifier: NewCombinedClusterPoolIdentifier(
				[]ClusterNodePoolIdentifier{
					NewNodeClassPoolIdentifier("test-class"),
					NewNodeDatacenterPoolIdentifier("test-dc"),
					NewMockPoolIdentifier(),
				},
				CombinedClusterPoolIdentifierAnd,
			),
			expectedValue: "node_class:test-class and datacenter:test-dc and mock_identifier:mock_value",
		},
		{
			name: "multiple identifiers with or",
			identifier: NewCombinedClusterPoolIdentifier(
				[]ClusterNodePoolIdentifier{
					NewNodeClassPoolIdentifier("test-class"),
					NewNodeDatacenterPoolIdentifier("test-dc"),
					NewMockPoolIdentifier(),
				},
				CombinedClusterPoolIdentifierOr,
			),
			expectedValue: "node_class:test-class or datacenter:test-dc or mock_identifier:mock_value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "combined_identifier", tc.identifier.Key())
			assert.Equal(t, tc.expectedValue, tc.identifier.Value())
		})
	}
}

func TestNewCombinedPoolIdentifier_NodeIsPoolMember(t *testing.T) {
	testCases := []struct {
		name       string
		identifier ClusterNodePoolIdentifier
		nodes      []*api.NodeListStub
		expected   []bool
	}{
		{
			name: "find node by class and datacenter",
			identifier: NewCombinedClusterPoolIdentifier(
				[]ClusterNodePoolIdentifier{
					NewNodeClassPoolIdentifier("test-class"),
					NewNodeDatacenterPoolIdentifier("test-dc"),
				},
				CombinedClusterPoolIdentifierAnd,
			),
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
			name: "find node by class or datacenter",
			identifier: NewCombinedClusterPoolIdentifier(
				[]ClusterNodePoolIdentifier{
					NewNodeClassPoolIdentifier("test-class"),
					NewNodeDatacenterPoolIdentifier("test-dc"),
				},
				CombinedClusterPoolIdentifierOr,
			),
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
			expected: []bool{true, true, true, true, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, node := range tc.nodes {
				got := tc.identifier.IsPoolMember(node)
				assert.Equal(t, tc.expected[i], got)
			}
		})
	}
}
