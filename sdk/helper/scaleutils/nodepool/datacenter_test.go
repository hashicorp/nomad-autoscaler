package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestNewNodeDatacenterPoolIdentifier(t *testing.T) {
	poolID := NewNodeDatacenterPoolIdentifier("eu-west-2a")
	assert.Equal(t, "datacenter", poolID.Key())
	assert.Equal(t, "eu-west-2a", poolID.Value())
}

func TestNodeDatacenterClusterPoolIdentifier_NodeIsPoolMember(t *testing.T) {
	testCases := []struct {
		inputPI        ClusterNodePoolIdentifier
		inputNode      *api.NodeListStub
		expectedOutput bool
		name           string
	}{
		{
			inputPI:        NewNodeDatacenterPoolIdentifier("dc1"),
			inputNode:      &api.NodeListStub{Datacenter: "dc1"},
			expectedOutput: true,
			name:           "el classico dc1",
		},
		{
			inputPI:        NewNodeDatacenterPoolIdentifier("dc1"),
			inputNode:      &api.NodeListStub{Datacenter: "eu-west-2a"},
			expectedOutput: false,
			name:           "non-matching datacenter",
		},
		{
			inputPI:        NewNodeDatacenterPoolIdentifier("eu-west-2a"),
			inputNode:      &api.NodeListStub{Datacenter: "eu-west-2b"},
			expectedOutput: false,
			name:           "non-matching close",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputPI.IsPoolMember(tc.inputNode), tc.name)
		})
	}
}
