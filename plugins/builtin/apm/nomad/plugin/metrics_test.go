package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateMetric(t *testing.T) {
	testCases := []struct {
		inputMetric    string
		expectedOutput error
		name           string
	}{
		{
			inputMetric:    "memory",
			expectedOutput: nil,
			name:           "memory metric",
		},
		{
			inputMetric:    "cpu",
			expectedOutput: nil,
			name:           "cpu metric",
		},
		{
			inputMetric:    "cost-of-server",
			expectedOutput: errors.New("invalid metric \"cost-of-server\", allowed values are cpu or memory"),
			name:           "invalid metric",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := validateMetric(tc.inputMetric)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
