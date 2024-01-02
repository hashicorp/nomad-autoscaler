// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_calculateTaskGroupResult(t *testing.T) {
	testCases := []struct {
		inputOp        string
		inputMetrics   []float64
		expectedOutput float64
		name           string
	}{
		{
			inputOp:        queryOpSum,
			inputMetrics:   []float64{76.34, 13.13, 24.50},
			expectedOutput: 113.97,
			name:           "sum operation",
		},
		{
			inputOp:        queryOpAvg,
			inputMetrics:   []float64{76.34, 13.13, 24.50},
			expectedOutput: 37.99,
			name:           "avg operation",
		},
		{
			inputOp:        queryOpMax,
			inputMetrics:   []float64{76.34, 13.13, 24.50, 76.33},
			expectedOutput: 76.34,
			name:           "max operation",
		},
		{
			inputOp:        queryOpMin,
			inputMetrics:   []float64{76.34, 13.13, 24.50, 13.14},
			expectedOutput: 13.13,
			name:           "min operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := calculateTaskGroupResult(tc.inputOp, tc.inputMetrics)
			assert.Equal(t, tc.expectedOutput, actualOutput[0].Value, tc.name)
		})
	}
}

func Test_parseTaskGroupQuery(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    *taskGroupQuery
		expectError bool
	}{
		{
			name:  "avg_cpu",
			input: "taskgroup_avg_cpu/group/job@default",
			expected: &taskGroupQuery{
				metric:    "cpu",
				namespace: "default",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "avg_cpu-allocated",
			input: "taskgroup_avg_cpu-allocated/group/job@ns",
			expected: &taskGroupQuery{
				metric:    "cpu-allocated",
				namespace: "ns",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "avg_memory",
			input: "taskgroup_avg_memory/group/job@dev",
			expected: &taskGroupQuery{
				metric:    "memory",
				namespace: "dev",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "avg_memory-allocated",
			input: "taskgroup_avg_memory-allocated/group/job@dev",
			expected: &taskGroupQuery{
				metric:    "memory-allocated",
				namespace: "dev",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "job with fwd slashes",
			input: "taskgroup_avg_cpu/group/my/super/job//@dev",
			expected: &taskGroupQuery{
				metric:    "cpu",
				namespace: "dev",
				job:       "my/super/job//",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "job with at signs",
			input: "taskgroup_avg_cpu/group/job@job@@dev",
			expected: &taskGroupQuery{
				metric:    "cpu",
				namespace: "dev",
				job:       "job@job@",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:        "empty query",
			input:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid query format",
			input:       "invalid",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "missing namspace",
			input:       "avg_cpu/group/job",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "missing job",
			input:       "avg_cpu/group",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid op_metric format",
			input:       "taskgroup_invalid/groups/job",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid operation",
			input:       "taskgroup_op_invalid/group/job",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseTaskGroupQuery(tc.input)

			assert.Equal(t, tc.expected, actual)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
