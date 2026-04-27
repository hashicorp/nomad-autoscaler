// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_parseNodePoolQuery(t *testing.T) {
	testCases := []struct {
		inputQuery          string
		expectedOutputQuery *nodePoolQuery
		expectError         error
		name                string
	}{
		{
			inputQuery: "node_percentage-allocated_memory/high-memory/class",
			expectedOutputQuery: &nodePoolQuery{
				metric:         "memory",
				poolIdentifier: nodepool.NewNodeClassPoolIdentifier("high-memory"),
				operation:      "percentage-allocated",
			},
			expectError: nil,
			name:        "node percentage-allocated memory",
		},
		{
			inputQuery: "node_percentage-allocated_cpu/high-compute/class",
			expectedOutputQuery: &nodePoolQuery{
				metric:         "cpu",
				poolIdentifier: nodepool.NewNodeClassPoolIdentifier("high-compute"),
				operation:      "percentage-allocated",
			},
			expectError: nil,
			name:        "node percentage-allocated cpu",
		},

		{
			inputQuery:          "",
			expectedOutputQuery: nil,
			expectError:         errors.New("expected <query>/<pool_identifier_value>/<pool_identifier_key>, received "),
			name:                "empty input query",
		},
		{
			inputQuery:          "invalid",
			expectedOutputQuery: nil,
			expectError:         errors.New("expected <query>/<pool_identifier_value>/<pool_identifier_key>, received invalid"),
			name:                "invalid input query format",
		},
		{
			inputQuery:          "node_percentage-allocated_cpu/class",
			expectedOutputQuery: nil,
			expectError:         errors.New("expected <query>/<pool_identifier_value>/<pool_identifier_key>, received node_percentage-allocated_cpu/class"),
			name:                "missing node pool identifier value",
		},
		{
			inputQuery:          "node_percentage-allocated_invalid/class/high-compute",
			expectedOutputQuery: nil,
			expectError:         errors.New("invalid metric \"invalid\", allowed values are: cpu, memory"),
			name:                "invalid metric",
		},
		{
			inputQuery:          "node_percentage-allocated_cpu-allocated/class/high-compute",
			expectedOutputQuery: nil,
			expectError:         errors.New("invalid metric \"cpu-allocated\", allowed values are: cpu, memory"),
			name:                "metric for task group queries only",
		},
		{
			inputQuery:          "node_invalid_cpu/class/high-compute",
			expectedOutputQuery: nil,
			expectError:         errors.New("invalid operation \"invalid\", allowed value is percentage-allocated"),
			name:                "invalid operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualQuery, actualError := parseNodePoolQuery(tc.inputQuery)
			assert.Equal(t, tc.expectedOutputQuery, actualQuery, tc.name)
			assert.Equal(t, tc.expectError, actualError, tc.name)
		})
	}
}

func Test_calculateNodePoolResult(t *testing.T) {
	testCases := []struct {
		inputAllocated   float64
		inputAllocatable float64
		expectedOutput   float64
		name             string
	}{
		{
			inputAllocated:   1000,
			inputAllocatable: 38400,
			expectedOutput:   2.6041666666666665,
			name:             "general example calculation 1",
		},
		{
			inputAllocated:   512,
			inputAllocatable: 16384,
			expectedOutput:   3.125,
			name:             "general example calculation 2",
		},
		{
			inputAllocated:   0,
			inputAllocatable: 16384,
			expectedOutput:   0,
			name:             "zero allocated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := calculateNodePoolResult(tc.inputAllocated, tc.inputAllocatable)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func Test_isServerTerminalStatus(t *testing.T) {
	testCases := []struct {
		inputAlloc     *api.Allocation
		expectedOutput bool
		name           string
	}{
		{
			inputAlloc:     &api.Allocation{DesiredStatus: "run"},
			expectedOutput: false,
			name:           "desired status run",
		},
		{
			inputAlloc:     &api.Allocation{DesiredStatus: "stop"},
			expectedOutput: true,
			name:           "desired status stop",
		},
		{
			inputAlloc:     &api.Allocation{DesiredStatus: "evict"},
			expectedOutput: true,
			name:           "desired status evict",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := isServerTerminalStatus(tc.inputAlloc)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func Test_isClientTerminalStatus(t *testing.T) {
	testCases := []struct {
		inputAlloc     *api.Allocation
		expectedOutput bool
		name           string
	}{
		{
			inputAlloc:     &api.Allocation{ClientStatus: "pending"},
			expectedOutput: false,
			name:           "client status pending",
		},
		{
			inputAlloc:     &api.Allocation{ClientStatus: "running"},
			expectedOutput: false,
			name:           "client status running",
		},
		{
			inputAlloc:     &api.Allocation{ClientStatus: "complete"},
			expectedOutput: true,
			name:           "client status complete",
		},
		{
			inputAlloc:     &api.Allocation{ClientStatus: "failed"},
			expectedOutput: true,
			name:           "client status failed",
		},
		{
			inputAlloc:     &api.Allocation{ClientStatus: "lost"},
			expectedOutput: true,
			name:           "client status lost",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := isClientTerminalStatus(tc.inputAlloc)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func TestAPMPlugin_getPoolResources(t *testing.T) {
	id := nodepool.NewNodeClassPoolIdentifier("1")
	cpu := 20
	mem := 15

	testCases := []struct {
		name          string
		config        map[string]string
		httpHandler   http.HandlerFunc
		expectError   bool
		expectedErr   string
		expectedCPU   int64
		expectedMemMB int64
	}{
		{
			name:        "draining node fails by default",
			config:      map[string]string{},
			expectError: true,
			expectedErr: "node node-draining is draining",
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-draining", NodeClass: "1", Drain: true, Status: api.NodeStatusReady},
						{ID: "node-ready", NodeClass: "1", Drain: false, SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
					})
				default:
					http.NotFound(w, r)
				}
			}),
		},
		{
			name: "draining node ignored when configured",
			config: map[string]string{
				"node_filter_ignore_drain": "true",
			},
			expectError:   false,
			expectedCPU:   20,
			expectedMemMB: 15,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-draining", NodeClass: "1", Drain: true, Status: api.NodeStatusReady},
						{ID: "node-ready", NodeClass: "1", Drain: false, SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
					})
				case "/v1/node/node-ready":
					_ = json.NewEncoder(w).Encode(&api.Node{
						ID: "node-ready",
						NodeResources: &api.NodeResources{
							Cpu:    api.NodeCpuResources{CpuShares: 300},
							Memory: api.NodeMemoryResources{MemoryMB: 256},
						},
						ReservedResources: &api.NodeReservedResources{
							Cpu:    api.NodeReservedCpuResources{CpuShares: 100},
							Memory: api.NodeReservedMemoryResources{MemoryMB: 64},
						},
					})
				case "/v1/node/node-ready/allocations":
					_ = json.NewEncoder(w).Encode([]*api.Allocation{
						{
							DesiredStatus: api.AllocDesiredStatusRun,
							ClientStatus:  api.AllocClientStatusRunning,
							Resources: &api.Resources{
								CPU:      &cpu,
								MemoryMB: &mem,
							},
						},
					})
				default:
					http.NotFound(w, r)
				}
			}),
		},
		{
			name:        "ready ineligible node excluded from pool totals",
			config:      map[string]string{},
			expectError: false,
			expectedCPU: 20,
			expectedMemMB: 15,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-ready-eligible", NodeClass: "1", Drain: false, SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
						{ID: "node-ready-ineligible", NodeClass: "1", Drain: false, SchedulingEligibility: api.NodeSchedulingIneligible, Status: api.NodeStatusReady},
					})
				case "/v1/node/node-ready-eligible":
					_ = json.NewEncoder(w).Encode(&api.Node{
						ID: "node-ready-eligible",
						NodeResources: &api.NodeResources{
							Cpu:    api.NodeCpuResources{CpuShares: 300},
							Memory: api.NodeMemoryResources{MemoryMB: 256},
						},
						ReservedResources: &api.NodeReservedResources{
							Cpu:    api.NodeReservedCpuResources{CpuShares: 100},
							Memory: api.NodeReservedMemoryResources{MemoryMB: 64},
						},
					})
				case "/v1/node/node-ready-eligible/allocations":
					_ = json.NewEncoder(w).Encode([]*api.Allocation{
						{
							DesiredStatus: api.AllocDesiredStatusRun,
							ClientStatus:  api.AllocClientStatusRunning,
							Resources: &api.Resources{
								CPU:      &cpu,
								MemoryMB: &mem,
							},
						},
					})
				case "/v1/node/node-ready-ineligible":
					_ = json.NewEncoder(w).Encode(&api.Node{
						ID: "node-ready-ineligible",
						NodeResources: &api.NodeResources{
							Cpu:    api.NodeCpuResources{CpuShares: 1000},
							Memory: api.NodeMemoryResources{MemoryMB: 1000},
						},
						ReservedResources: &api.NodeReservedResources{
							Cpu:    api.NodeReservedCpuResources{CpuShares: 0},
							Memory: api.NodeReservedMemoryResources{MemoryMB: 0},
						},
					})
				case "/v1/node/node-ready-ineligible/allocations":
					ineligibleCPU := 500
					ineligibleMem := 500
					_ = json.NewEncoder(w).Encode([]*api.Allocation{
						{
							DesiredStatus: api.AllocDesiredStatusRun,
							ClientStatus:  api.AllocClientStatusRunning,
							Resources: &api.Resources{
								CPU:      &ineligibleCPU,
								MemoryMB: &ineligibleMem,
							},
						},
					})
				default:
					http.NotFound(w, r)
				}
			}),
		},
		{
			name: "invalid node filter option",
			config: map[string]string{
				"node_filter_ignore_drain": "not-a-bool",
			},
			expectError: true,
			expectedErr: "failed to parse node filter options",
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-ready", NodeClass: "1", SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
					})
				default:
					http.NotFound(w, r)
				}
			}),
		},
		{
			name:        "missing specific node resources",
			config:      map[string]string{},
			expectError: true,
			expectedErr: "node resources are incomplete",
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-missing-res", NodeClass: "1", SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
					})
				case "/v1/node/node-missing-res":
					_ = json.NewEncoder(w).Encode(&api.Node{ID: "node-missing-res"})
				default:
					http.NotFound(w, r)
				}
			}),
		},
		{
			name:        "missing specific allocation resources",
			config:      map[string]string{},
			expectError: true,
			expectedErr: "allocation resources are incomplete",
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/nodes":
					_ = json.NewEncoder(w).Encode([]*api.NodeListStub{
						{ID: "node-missing-alloc-res", NodeClass: "1", SchedulingEligibility: api.NodeSchedulingEligible, Status: api.NodeStatusReady},
					})
				case "/v1/node/node-missing-alloc-res":
					_ = json.NewEncoder(w).Encode(&api.Node{
						ID:                "node-missing-alloc-res",
						NodeResources:     &api.NodeResources{},
						ReservedResources: &api.NodeReservedResources{},
					})
				case "/v1/node/node-missing-alloc-res/allocations":
					_ = json.NewEncoder(w).Encode([]*api.Allocation{
						{
							DesiredStatus: api.AllocDesiredStatusRun,
							ClientStatus:  api.AllocClientStatusRunning,
						},
					})
				default:
					http.NotFound(w, r)
				}
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.httpHandler)
			defer ts.Close()

			client, err := api.NewClient(&api.Config{Address: ts.URL})
			assert.NoError(t, err)

			p := &APMPlugin{client: client, config: tc.config}
			resp, err := p.getPoolResources(id)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedErr != "" {
					assert.ErrorContains(t, err, tc.expectedErr)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tc.expectedCPU, resp.allocated.cpu)
			assert.Equal(t, tc.expectedMemMB, resp.allocated.mem)
		})
	}
}
