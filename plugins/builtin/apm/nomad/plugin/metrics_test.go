package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_calculateResult(t *testing.T) {
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
			actualOutput := calculateResult(tc.inputOp, tc.inputMetrics)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func Test_parseQuery(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    *query
		expectError bool
	}{
		{
			name:  "avg_cpu",
			input: "avg_cpu/group/job",
			expected: &query{
				metric:    "cpu",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "avg_memory",
			input: "avg_memory/group/job",
			expected: &query{
				metric:    "memory",
				job:       "job",
				group:     "group",
				operation: "avg",
			},
			expectError: false,
		},
		{
			name:  "job with fwd slashes",
			input: "avg_cpu/group/my/super/job//",
			expected: &query{
				metric:    "cpu",
				job:       "my/super/job//",
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
			name:        "missing job",
			input:       "avg_cpu/group",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid op_metric format",
			input:       "invalid/groups/job",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid operation",
			input:       "op_invalid/group/job",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseQuery(tc.input)

			assert.Equal(t, tc.expected, actual)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
