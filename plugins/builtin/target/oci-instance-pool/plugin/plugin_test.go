// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"strconv"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/shoenig/test"
)

func TestTargetPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputCurrent         int64
		inputStrategyDesired int64
		expectedOutputNum    int64
		expectedOutputString string
		name                 string
	}{
		{
			inputCurrent:         10,
			inputStrategyDesired: 11,
			expectedOutputNum:    11,
			expectedOutputString: "out",
			name:                 "scale out desired",
		},
		{
			inputCurrent:         10,
			inputStrategyDesired: 9,
			expectedOutputNum:    1,
			expectedOutputString: "in",
			name:                 "scale in desired",
		},
		{
			inputCurrent:         10,
			inputStrategyDesired: 10,
			expectedOutputNum:    0,
			expectedOutputString: "",
			name:                 "scale not desired",
		},
	}

	tp := TargetPlugin{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNum, actualString := tp.calculateDirection(tc.inputCurrent, tc.inputStrategyDesired)
			test.Eq(t, tc.expectedOutputNum, actualNum)
			test.Eq(t, tc.expectedOutputString, actualString)
		})
	}
}

func TestTargetPlugin_SetConfig_RetryAttempts(t *testing.T) {
	mockPlugin := &TargetPlugin{
		logger: hclog.NewNullLogger(),
	}
	mockPlugin.setupOCIClientFunc = func(config map[string]string) error {
		return nil
	}

	t.Run("default value is used when not provided", func(t *testing.T) {
		err := mockPlugin.SetConfig(map[string]string{})
		test.NoError(t, err)

		defaultValue, _ := strconv.Atoi(configValueRetryAttemptsDefault)
		test.Eq(t, defaultValue, mockPlugin.retryAttempts)
	})

	t.Run("valid custom value is used", func(t *testing.T) {
		err := mockPlugin.SetConfig(map[string]string{
			configKeyRetryAttempts: "20",
		})
		test.NoError(t, err)
		test.Eq(t, 20, mockPlugin.retryAttempts)
	})

	t.Run("invalid value returns an error", func(t *testing.T) {
		err := mockPlugin.SetConfig(map[string]string{
			configKeyRetryAttempts: "not-a-number",
		})
		test.ErrorContains(t, err, "invalid value for retry_attempts")
	})
}
