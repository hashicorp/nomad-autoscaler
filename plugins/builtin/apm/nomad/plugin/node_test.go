// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
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
