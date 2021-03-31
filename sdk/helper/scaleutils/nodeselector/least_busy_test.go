package nodeselector

import (
	"sort"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_leastBusyClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "least_busy", newLeastBusyClusterScaleInNodeSelector(nil, nil).Name())
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
