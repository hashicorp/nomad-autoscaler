// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0
// TODO: Remove this file and all references in next Release.

package nodepool

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func Test_NewCombinedClusterPoolIdentifier_AND(t *testing.T) {
	classID := NewNodeClassPoolIdentifier("high-memory")
	dcID := NewNodeDatacenterPoolIdentifier("dc1")

	combined := NewCombinedClusterPoolIdentifier(
		[]ClusterNodePoolIdentifier{classID, dcID},
		CombinedClusterPoolIdentifierAnd,
	)

	testCases := []struct {
		name     string
		node     *api.NodeListStub
		expected bool
	}{
		{
			name:     "matches both → true",
			node:     &api.NodeListStub{NodeClass: "high-memory", Datacenter: "dc1"},
			expected: true,
		},
		{
			name:     "matches class only → false",
			node:     &api.NodeListStub{NodeClass: "high-memory", Datacenter: "dc2"},
			expected: false,
		},
		{
			name:     "matches dc only → false",
			node:     &api.NodeListStub{NodeClass: "low-memory", Datacenter: "dc1"},
			expected: false,
		},
		{
			name:     "matches neither → false",
			node:     &api.NodeListStub{NodeClass: "low-memory", Datacenter: "dc2"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, combined.IsPoolMember(tc.node))
		})
	}
}

func Test_NewCombinedClusterPoolIdentifier_OR(t *testing.T) {
	classID := NewNodeClassPoolIdentifier("high-memory")
	dcID := NewNodeDatacenterPoolIdentifier("dc1")

	combined := NewCombinedClusterPoolIdentifier(
		[]ClusterNodePoolIdentifier{classID, dcID},
		CombinedClusterPoolIdentifierOr,
	)

	testCases := []struct {
		name     string
		node     *api.NodeListStub
		expected bool
	}{
		{
			name:     "matches both → true",
			node:     &api.NodeListStub{NodeClass: "high-memory", Datacenter: "dc1"},
			expected: true,
		},
		{
			name:     "matches class only → true",
			node:     &api.NodeListStub{NodeClass: "high-memory", Datacenter: "dc2"},
			expected: true,
		},
		{
			name:     "matches dc only → true",
			node:     &api.NodeListStub{NodeClass: "low-memory", Datacenter: "dc1"},
			expected: true,
		},
		{
			name:     "matches neither → false",
			node:     &api.NodeListStub{NodeClass: "low-memory", Datacenter: "dc2"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, combined.IsPoolMember(tc.node))
		})
	}
}

func Test_NewCombinedClusterPoolIdentifier_Empty(t *testing.T) {
	combined := NewCombinedClusterPoolIdentifier(
		[]ClusterNodePoolIdentifier{},
		CombinedClusterPoolIdentifierAnd,
	)

	// Empty AND list: isMember starts true, no iterations, returns true.
	must.True(t, combined.IsPoolMember(&api.NodeListStub{NodeClass: "x", Datacenter: "dc1"}))
	must.Eq(t, "combined_identifier", combined.Key())
	must.Eq(t, "", combined.Value())
}

func Test_NewCombinedClusterPoolIdentifier_KeyValue(t *testing.T) {
	classID := NewNodeClassPoolIdentifier("high-memory")
	dcID := NewNodeDatacenterPoolIdentifier("dc1")

	combined := NewCombinedClusterPoolIdentifier(
		[]ClusterNodePoolIdentifier{classID, dcID},
		CombinedClusterPoolIdentifierAnd,
	)

	must.Eq(t, "combined_identifier", combined.Key())
	must.Eq(t, "node_class:high-memory and datacenter:dc1", combined.Value())
}
