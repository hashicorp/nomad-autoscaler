// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestNewNodePoolClusterIdentifier(t *testing.T) {
	poolID := NewNodePoolClusterPoolIdentifier("gpu")
	must.Eq(t, "node_pool", poolID.Key())
	must.Eq(t, "gpu", poolID.Value())
}

func TestNodePoolClusterPoolIdentifier_NodeIsPoolMember(t *testing.T) {
	testCases := []struct {
		inputPI        ClusterNodePoolIdentifier
		inputNode      *api.NodeListStub
		expectedOutput bool
		name           string
	}{
		{
			inputPI:        NewNodePoolClusterPoolIdentifier("foo"),
			inputNode:      &api.NodeListStub{NodePool: "bar"},
			expectedOutput: false,
			name:           "non-matched non-empty class",
		},
		{
			inputPI:        NewNodePoolClusterPoolIdentifier("foo"),
			inputNode:      &api.NodeListStub{NodePool: "foo"},
			expectedOutput: true,
			name:           "matched class",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputPI.IsPoolMember(tc.inputNode))
		})
	}
}
