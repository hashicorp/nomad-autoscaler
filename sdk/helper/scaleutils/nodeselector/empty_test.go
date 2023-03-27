// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"fmt"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_emptyClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "empty", newEmptyClusterScaleInNodeSelector(nil, nil, false).Name())
	assert.Equal(t, "empty_ignore_system", newEmptyClusterScaleInNodeSelector(nil, nil, true).Name())
}

func Test_emptyClusterScaleInNodeSelectorSelect(t *testing.T) {
	// Start a test server that reads node allocations from disk.
	ts, tsClose := NodeAllocsTestServer(t, "cluster1")
	defer tsClose()

	// Create Nomad client pointing to the test server.
	cfg := api.DefaultConfig()
	cfg.Address = ts.URL
	client, err := api.NewClient(cfg)
	if err != nil {
		t.Errorf("failed to create Nomad client: %v", err)
	}

	// Nodes workloads:
	//   * 151be1dc: 1 system job, 1 running job, and 1 complete job.
	//   * b2d5bbb6: 1 system job and 1 complete job.
	//   * b535e699: 1 complete job.
	testCases := []struct {
		name                    string
		nodes                   []*api.NodeListStub
		num                     int
		expectedIDs             []string
		expectedIDsIgnoreSystem []string
	}{
		{
			name:                    "empty list and select 0",
			nodes:                   []*api.NodeListStub{},
			num:                     0,
			expectedIDs:             nil,
			expectedIDsIgnoreSystem: nil,
		},
		{
			name:                    "empty list and select more than 0",
			nodes:                   []*api.NodeListStub{},
			num:                     5,
			expectedIDs:             nil,
			expectedIDsIgnoreSystem: nil,
		},
		{
			name: "select 1 node",
			nodes: []*api.NodeListStub{
				{ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96"},
				{ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe"},
				{ID: "b535e699-1112-c379-c020-ebd80fdd9f09"},
			},
			num: 1,
			expectedIDs: []string{
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
			expectedIDsIgnoreSystem: []string{
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
			},
		},
		{
			name: "select 2 nodes",
			nodes: []*api.NodeListStub{
				{ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96"},
				{ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe"},
				{ID: "b535e699-1112-c379-c020-ebd80fdd9f09"},
			},
			num: 2,
			expectedIDs: []string{
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
			expectedIDsIgnoreSystem: []string{
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
		{
			name: "select more than 2 nodes",
			nodes: []*api.NodeListStub{
				{ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96"},
				{ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe"},
				{ID: "b535e699-1112-c379-c020-ebd80fdd9f09"},
			},
			num: 5,
			expectedIDs: []string{
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
			expectedIDsIgnoreSystem: []string{
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
	}

	for _, tc := range testCases {
		for _, ignoreSystem := range []bool{true, false} {
			// Create a `empty` selector instance.
			selector := newEmptyClusterScaleInNodeSelector(client, hclog.NewNullLogger(), ignoreSystem)
			name := fmt.Sprintf("%s/%s", selector.Name(), tc.name)
			t.Run(name, func(t *testing.T) {
				expectedIDs := tc.expectedIDs
				if ignoreSystem {
					expectedIDs = tc.expectedIDsIgnoreSystem
				}

				got := selector.Select(tc.nodes, tc.num)
				var gotIDs []string
				for _, n := range got {
					gotIDs = append(gotIDs, n.ID)
				}

				assert.ElementsMatch(t, expectedIDs, gotIDs, "expected A, got B")
			})
		}
	}
}
