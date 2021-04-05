package nodeselector

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_emptyClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "empty", newEmptyClusterScaleInNodeSelector(nil, nil).Name())
}

func Test_emptyClusterScaleInNodeSelectSelect(t *testing.T) {
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

	// Create a `empty` selector instance.
	selector := newEmptyClusterScaleInNodeSelector(client, hclog.NewNullLogger())

	testCases := []struct {
		name        string
		nodes       []*api.NodeListStub
		num         int
		expectedIDs []string
	}{
		{
			name:        "empty list and select 0",
			nodes:       []*api.NodeListStub{},
			num:         0,
			expectedIDs: nil,
		},
		{
			name:        "empty list and select more than 0",
			nodes:       []*api.NodeListStub{},
			num:         5,
			expectedIDs: nil,
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
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := selector.Select(tc.nodes, tc.num)
			var gotIDs []string
			for _, n := range got {
				gotIDs = append(gotIDs, n.ID)
			}
			assert.ElementsMatch(t, tc.expectedIDs, gotIDs)
		})
	}
}
