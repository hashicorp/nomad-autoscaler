package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputAsgDesired      int64
		inputStrategyDesired int64
		expectedOutputNum    int64
		expectedOutputString string
		name                 string
	}{
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 11,
			expectedOutputNum:    11,
			expectedOutputString: "out",
			name:                 "scale out desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 9,
			expectedOutputNum:    1,
			expectedOutputString: "in",
			name:                 "scale in desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 10,
			expectedOutputNum:    0,
			expectedOutputString: "",
			name:                 "scale not desired",
		},
	}

	tp := TargetPlugin{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNum, actualString := tp.calculateDirection(tc.inputAsgDesired, tc.inputStrategyDesired)
			assert.Equal(t, tc.expectedOutputNum, actualNum, tc.name)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
		})
	}
}
