package scaleutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_autoscalerNodeID(t *testing.T) {
	testCases := []struct {
		envVar               bool
		expectedOutputString string
		expectedOutputError  error
		name                 string
	}{
		{
			envVar:               false,
			expectedOutputString: "",
			expectedOutputError:  nil,
			name:                 "no alloc ID found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualString, actualError := autoscalerNodeID(nil)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
		})
	}
}
