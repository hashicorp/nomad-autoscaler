package policystorage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseStrategy(t *testing.T) {
	testCases := []struct {
		inputStrategy  interface{}
		expectedOutput *Strategy
	}{
		{
			inputStrategy: []interface{}{
				map[string]interface{}{
					"name": "target-value",
					"config": []interface{}{
						map[string]interface{}{"target": float64(20)},
					},
				},
			},
			expectedOutput: &Strategy{
				Name:   "target-value",
				Config: map[string]string{"target": "20"},
			},
		},
	}

	for _, tc := range testCases {
		actualOutput := parseStrategy(tc.inputStrategy)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

func Test_parseTarget(t *testing.T) {
	testCases := []struct {
		inputTarget    interface{}
		expectedOutput *Target
		name           string
	}{
		{
			inputTarget:    nil,
			expectedOutput: &Target{},
			name:           "nil passed target interface",
		},
	}

	for _, tc := range testCases {
		actualOutput := parseTarget(tc.inputTarget)
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
	}
}
