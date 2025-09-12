// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shoenig/test"
)

func Test_retry(t *testing.T) {
	testCases := []struct {
		inputContext   context.Context
		inputInterval  time.Duration
		inputRetry     int
		inputFunc      retryFunc
		expectedOutput error
		name           string
	}{
		{
			inputContext:  context.Background(),
			inputInterval: 1 * time.Millisecond,
			inputRetry:    1,
			inputFunc: func(ctx context.Context) (stop bool, err error) {
				return true, nil
			},
			expectedOutput: nil,
			name:           "successful function first time",
		},
		{
			inputContext:  context.Background(),
			inputInterval: 1 * time.Millisecond,
			inputRetry:    1,
			inputFunc: func(ctx context.Context) (stop bool, err error) {
				return false, errors.New("error")
			},
			expectedOutput: errors.New("reached retry limit"),
			name:           "function never successful and reaches retry attempts limit",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := retry(tc.inputContext, tc.inputInterval, tc.inputRetry, tc.inputFunc)
			test.Eq(t, tc.expectedOutput, actualOutput, test.Sprint(tc.name))
		})
	}
}
