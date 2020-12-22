package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateMetric(t *testing.T) {
	testCases := []struct {
		inputMetric       string
		inputValidMetrics []string
		expectedOutput    error
		name              string
	}{
		{
			inputMetric:    "memory",
			inputValidMetrics: []string{queryMetricCPU, queryMetricCPUAllocated, queryMetricMem},
			expectedOutput: nil,
			name:           "memory metric",
		},
		{
			inputMetric:    "cpu",
			inputValidMetrics: []string{queryMetricCPU, queryMetricCPUAllocated, queryMetricMem},
			expectedOutput: nil,
			name:           "cpu metric",
		},
		{
			inputMetric:    "cpu-allocated",
			inputValidMetrics: []string{queryMetricCPU, queryMetricCPUAllocated, queryMetricMem},
			expectedOutput: nil,
			name:           "cpu-allocated metric",
		},
		{
			inputMetric:    "cost-of-server",
			inputValidMetrics: []string{queryMetricCPU, queryMetricCPUAllocated, queryMetricMem},
			expectedOutput: errors.New("invalid metric \"cost-of-server\", allowed values are: cpu, cpu-allocated, memory"),
			name:           "invalid metric",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := validateMetric(tc.inputMetric, tc.inputValidMetrics)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
