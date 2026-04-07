// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_retry(t *testing.T) {
	testCases := []struct {
		inputContext  context.Context
		inputInterval time.Duration
		inputRetry    int
		inputFunc     retryFunc
		expectedError string
		name          string
	}{
		{
			inputContext:  context.Background(),
			inputInterval: 1 * time.Millisecond,
			inputRetry:    1,
			inputFunc: func(ctx context.Context) (stop bool, err error) {
				return true, nil
			},
			expectedError: "",
			name:          "successful function first time",
		},
		{
			inputContext:  context.Background(),
			inputInterval: 1 * time.Millisecond,
			inputRetry:    1,
			inputFunc: func(ctx context.Context) (stop bool, err error) {
				return false, errors.New("error")
			},
			expectedError: "reached retry limit: error",
			name:          "function never successful and reaches retry limit",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := retry(tc.inputContext, tc.inputInterval, tc.inputRetry, tc.inputFunc)
			if tc.expectedError == "" {
				assert.NoError(t, actualOutput, tc.name)
				return
			}

			assert.EqualError(t, actualOutput, tc.expectedError, tc.name)
		})
	}
}
