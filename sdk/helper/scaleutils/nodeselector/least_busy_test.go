package nodeselector

import (
	"sort"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_leastBusyClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "least_busy", newLeastBusyClusterScaleInNodeSelector(nil, nil).Name())
}

func Test_leastBusyClusterScaleInNodeSelectorSelect(t *testing.T) {
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

	// Create a `least_busy` selector instance.
	selector := newLeastBusyClusterScaleInNodeSelector(client, hclog.NewNullLogger())

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
				{
					ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b535e699-1112-c379-c020-ebd80fdd9f09",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
			},
			num: 1,
			expectedIDs: []string{
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
		{
			name: "select 1 node with 1st node super chill",
			nodes: []*api.NodeListStub{
				{
					ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 25000000,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 198500000,
						},
					},
				},
				{
					ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				// Ignore b535e699 since it doesn't have any load.
			},
			num: 1,
			expectedIDs: []string{
				"151be1dc-92e7-a488-f7dc-49faaf5a4c96",
			},
		},
		{
			name: "select 2 nodes",
			nodes: []*api.NodeListStub{
				{
					ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b535e699-1112-c379-c020-ebd80fdd9f09",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
			},
			num: 2,
			expectedIDs: []string{
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
		{
			name: "select 3 nodes",
			nodes: []*api.NodeListStub{
				{
					ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b535e699-1112-c379-c020-ebd80fdd9f09",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
			},
			num: 3,
			expectedIDs: []string{
				"151be1dc-92e7-a488-f7dc-49faaf5a4c96",
				"b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
				"b535e699-1112-c379-c020-ebd80fdd9f09",
			},
		},
		{
			name: "select more than 3 nodes",
			nodes: []*api.NodeListStub{
				{
					ID: "151be1dc-92e7-a488-f7dc-49faaf5a4c96",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b2d5bbb6-a61d-ee1f-a896-cbc6efe4f7fe",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
				{
					ID: "b535e699-1112-c379-c020-ebd80fdd9f09",
					NodeResources: &api.NodeResources{
						Cpu: api.NodeCpuResources{
							CpuShares: 2500,
						},
						Memory: api.NodeMemoryResources{
							MemoryMB: 1985,
						},
					},
				},
			},
			num: 10,
			expectedIDs: []string{
				"151be1dc-92e7-a488-f7dc-49faaf5a4c96",
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
			assert.ElementsMatch(t, tc.expectedIDs, gotIDs, "expected A, got B")
		})
	}
}

func Test_computeNodeAllocatedResources(t *testing.T) {
	testCases := []struct {
		inputAllocs    []*api.Allocation
		expectedOutput *api.NodeResources
		name           string
	}{
		{
			inputAllocs: []*api.Allocation{
				{
					ClientStatus: "running",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
				{
					ClientStatus: "complete",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
				{
					ClientStatus: "failed",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
				{
					ClientStatus: "pending",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
				{
					ClientStatus: "running",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
				{
					ClientStatus: "lost",
					Resources: &api.Resources{
						CPU:      ptr.IntToPtr(100),
						MemoryMB: ptr.IntToPtr(50),
					},
				},
			},
			expectedOutput: &api.NodeResources{
				Cpu:    api.NodeCpuResources{CpuShares: 200},
				Memory: api.NodeMemoryResources{MemoryMB: 100},
			},
			name: "multiple different allocs states",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, computeNodeAllocatedResources(tc.inputAllocs), tc.name)
		})
	}
}

func Test_computeNodeTotalResources(t *testing.T) {
	testCases := []struct {
		inputNode      *api.NodeListStub
		expectedOutput *api.NodeResources
		name           string
	}{
		{
			inputNode: &api.NodeListStub{
				NodeResources: &api.NodeResources{
					Cpu:    api.NodeCpuResources{CpuShares: 38400},
					Memory: api.NodeMemoryResources{MemoryMB: 16777216},
				},
				ReservedResources: nil,
			},
			expectedOutput: &api.NodeResources{
				Cpu:    api.NodeCpuResources{CpuShares: 38400},
				Memory: api.NodeMemoryResources{MemoryMB: 16777216},
			},
			name: "empty reserved resources",
		},
		{
			inputNode: &api.NodeListStub{
				NodeResources: &api.NodeResources{
					Cpu:    api.NodeCpuResources{CpuShares: 38400},
					Memory: api.NodeMemoryResources{MemoryMB: 16777216},
				},
				ReservedResources: &api.NodeReservedResources{
					Cpu:    api.NodeReservedCpuResources{CpuShares: 400},
					Memory: api.NodeReservedMemoryResources{MemoryMB: 200},
				},
			},
			expectedOutput: &api.NodeResources{
				Cpu:    api.NodeCpuResources{CpuShares: 38000},
				Memory: api.NodeMemoryResources{MemoryMB: 16777016},
			},
			name: "non-nil reserved resources",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, computeNodeTotalResources(tc.inputNode), tc.name)
		})
	}
}

func Test_computeNodeAllocatedPercentage(t *testing.T) {
	testCases := []struct {
		inputAllocated int64
		inputTotal     int64
		expectedOutput float64
		name           string
	}{
		{
			inputAllocated: 1000,
			inputTotal:     10000,
			expectedOutput: 10,
			name:           "generic non-zero",
		},
		{
			inputAllocated: 0,
			inputTotal:     10000,
			expectedOutput: 0,
			name:           "zero allocated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, computeNodeAllocatedPercentage(tc.inputAllocated, tc.inputTotal), tc.name)
		})
	}
}

func Test_nodeResourceConsumption(t *testing.T) {
	testCases := []struct {
		inputEntries []*nodeResourceStats
		popWanted    int
		popExpected  []*api.NodeListStub
		name         string
	}{
		{
			inputEntries: []*nodeResourceStats{
				{
					consumed: 0,
					node:     &api.NodeListStub{ID: "67533726-498d-08d4-ff68-90633ea6e5a6"},
				},
				{
					consumed: 10,
					node:     &api.NodeListStub{ID: "67533726-498d-08d4-ff68-90633ea6e5a7"},
				},
				{
					consumed: 11,
					node:     &api.NodeListStub{ID: "67533726-498d-08d4-ff68-90633ea6e5a8"},
				},
			},
			popWanted: 1,
			popExpected: []*api.NodeListStub{
				{ID: "67533726-498d-08d4-ff68-90633ea6e5a6"},
			},
			name: "multiple entries",
		},
		{
			inputEntries: []*nodeResourceStats{
				{
					consumed: 10,
					node:     &api.NodeListStub{ID: "67533726-498d-08d4-ff68-90633ea6e5a7"},
				},
			},
			popWanted: 1,
			popExpected: []*api.NodeListStub{
				{ID: "67533726-498d-08d4-ff68-90633ea6e5a7"},
			},
			name: "single entry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			h := make(nodeResourceConsumption, len(tc.inputEntries))
			for i, entry := range tc.inputEntries {
				h[i] = entry
			}
			sort.Sort(h)

			actualPop := make([]*api.NodeListStub, tc.popWanted)
			for i := 0; i <= tc.popWanted-1; i++ {
				actualPop[i] = h[i].node
			}
			assert.ElementsMatch(t, tc.popExpected, actualPop, tc.name)
		})
	}
}
