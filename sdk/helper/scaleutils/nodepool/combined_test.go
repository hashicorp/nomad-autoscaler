// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
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
				assert.Equal(t, tc.expected[i], got, tc.name)
			}
		})
	}
}

func TestEncodeCombinedQueryIdentifiers(t *testing.T) {
	testCases := []struct {
		name     string
		ids      []ClusterNodePoolIdentifier
		expected string
	}{
		{
			name: "single identifier",
			ids: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("gpu"),
			},
			expected: "node_class=gpu",
		},
		{
			name: "two identifiers",
			ids: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("hashistack"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			expected: "node_class=hashistack+datacenter=dc1",
		},
		{
			name: "three identifiers",
			ids: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("gpu"),
				NewNodeDatacenterPoolIdentifier("us-east-1a"),
				NewNodePoolClusterPoolIdentifier("default"),
			},
			expected: "node_class=gpu+datacenter=us-east-1a+node_pool=default",
		},
		{
			name: "value with special characters",
			ids: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("special+class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
			expected: "node_class=special%2Bclass+datacenter=dc1",
		},
		{
			name: "value with equals sign",
			ids: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("key=value"),
			},
			expected: "node_class=key%3Dvalue",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := EncodeCombinedQueryIdentifiers(tc.ids)
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestDecodeCombinedQueryIdentifiers(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedIDs []ClusterNodePoolIdentifier
		expectError bool
		errorMsg    string
	}{
		{
			name:  "single node_class",
			input: "node_class=gpu",
			expectedIDs: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("gpu"),
			},
		},
		{
			name:  "node_class and datacenter",
			input: "node_class=hashistack+datacenter=dc1",
			expectedIDs: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("hashistack"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
		},
		{
			name:  "URL-encoded value with plus",
			input: "node_class=special%2Bclass+datacenter=dc1",
			expectedIDs: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("special+class"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
		},
		{
			name:  "URL-encoded value with equals",
			input: "node_class=key%3Dvalue",
			expectedIDs: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("key=value"),
			},
		},
		{
			name:  "legacy class key",
			input: "class=hashistack+datacenter=dc1",
			expectedIDs: []ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier("hashistack"),
				NewNodeDatacenterPoolIdentifier("dc1"),
			},
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

func TestCombinedPoolIdentifier_Interface(t *testing.T) {
	ids := []ClusterNodePoolIdentifier{
		NewNodeClassPoolIdentifier("gpu"),
		NewNodeDatacenterPoolIdentifier("dc1"),
	}
	combined := NewCombinedClusterPoolIdentifier(ids, CombinedClusterPoolIdentifierAnd)

	// Verify it satisfies CombinedPoolIdentifier interface
	cpi, ok := combined.(CombinedPoolIdentifier)
	must.True(t, ok)
	must.Eq(t, ids, cpi.Identifiers())
}
