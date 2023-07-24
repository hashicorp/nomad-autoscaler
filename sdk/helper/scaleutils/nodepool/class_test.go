// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestNewNodeClassPoolIdentifier(t *testing.T) {
	poolID := NewNodeClassPoolIdentifier("high-memory-ridiculous-price", false)
	assert.Equal(t, "node_class", poolID.Key())
	assert.Equal(t, "high-memory-ridiculous-price", poolID.Value())
}

func TestNodeClassClusterPoolIdentifier_NodeIsPoolMember(t *testing.T) {
	testCases := []struct {
		inputPI        ClusterNodePoolIdentifier
		inputNode      *api.NodeListStub
		expectedOutput bool
		name           string
	}{
		{
			inputPI:        NewNodeClassPoolIdentifier("foo", false),
			inputNode:      &api.NodeListStub{NodeClass: ""},
			expectedOutput: false,
			name:           "non-matched empty class",
		},
		{
			inputPI:        NewNodeClassPoolIdentifier("foo", false),
			inputNode:      &api.NodeListStub{NodeClass: "bar"},
			expectedOutput: false,
			name:           "non-matched non-empty class",
		},
		{
			inputPI:        NewNodeClassPoolIdentifier("autoscaler-default-pool", false),
			inputNode:      &api.NodeListStub{NodeClass: ""},
			expectedOutput: true,
			name:           "matched default class",
		},
		{
			inputPI:        NewNodeClassPoolIdentifier("foo", false),
			inputNode:      &api.NodeListStub{NodeClass: "foo"},
			expectedOutput: true,
			name:           "matched class",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputPI.IsPoolMember(tc.inputNode), tc.name)
		})
	}
}
