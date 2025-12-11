// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"strconv"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/test"
)

func TestTargetPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputMigTarget       int64
		inputStrategyDesired int64
		expectedOutputNum    int64
		expectedOutputString string
		name                 string
	}{
		{
			inputMigTarget:       10,
			inputStrategyDesired: 11,
			expectedOutputNum:    11,
			expectedOutputString: "out",
			name:                 "scale out desired",
		},
		{
			inputMigTarget:       10,
			inputStrategyDesired: 9,
			expectedOutputNum:    1,
			expectedOutputString: "in",
			name:                 "scale in desired",
		},
		{
			inputMigTarget:       10,
			inputStrategyDesired: 10,
			expectedOutputNum:    0,
			expectedOutputString: "",
			name:                 "scale not desired",
		},
	}

	tp := TargetPlugin{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNum, actualString := tp.calculateDirection(tc.inputMigTarget, tc.inputStrategyDesired)
			test.Eq(t, tc.expectedOutputNum, actualNum)
			test.Eq(t, tc.expectedOutputString, actualString)
		})
	}
}

func TestTargetPlugin_SetConfig_RetryAttempts(t *testing.T) {
	mockPlugin := &TargetPlugin{
		logger: hclog.NewNullLogger(),
	}
	mockPlugin.setupGCEClientsFunc = func(config map[string]string) error {
		return nil
	}

	// Test case for the default value.
	t.Run("default value is used when not provided", func(t *testing.T) {
		config := map[string]string{}
		err := mockPlugin.SetConfig(config)
		test.NoError(t, err)

		defaultValue, _ := strconv.Atoi(configValueRetryAttemptsDefault)
		test.Eq(t, defaultValue, mockPlugin.retryAttempts, test.Sprint("expected default retry attempts"))
	})

	// Test case for a valid custom retry attempts value.
	t.Run("valid custom value is used", func(t *testing.T) {
		customAttempts := 20
		config := map[string]string{
			configKeyRetryAttempts: strconv.Itoa(customAttempts),
		}
		err := mockPlugin.SetConfig(config)
		test.NoError(t, err, test.Sprint("expected no error for valid config"))
		test.Eq(t, customAttempts, mockPlugin.retryAttempts, test.Sprint("expected custom retry attempts"))
	})

	// Test case for an invalid retry attempts value (non-integer).
	t.Run("invalid value returns an error", func(t *testing.T) {
		invalidConfig := map[string]string{
			configKeyRetryAttempts: "not-a-number",
		}
		err := mockPlugin.SetConfig(invalidConfig)
		test.ErrorContains(t, err, "invalid value for retry_attempts", test.Sprint("expected error for invalid config (non-integer)"))
	})
}
