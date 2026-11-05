// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shoenig/test"
)

func Test_retry(t *testing.T) {
	testCases := []struct {
		name           string
		inputContext   context.Context
		inputInterval  time.Duration
		inputAttempts  int
		inputFunc      retryFunc
		expectedOutput error
	}{
		{
			name:          "succeeds immediately on first call",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 5,
			inputFunc: func(_ context.Context) (bool, error) {
				return true, nil
			},
			expectedOutput: nil,
		},
		{
			name:          "succeeds after retries",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 10,
			inputFunc: func() retryFunc {
				calls := 0
				return func(_ context.Context) (bool, error) {
					calls++
					if calls < 4 {
						return false, errors.New("not ready")
					}
					return true, nil
				}
			}(),
			expectedOutput: nil,
		},
		{
			name:          "exhausts retry limit and returns reached retry limit",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 3,
			inputFunc: func(_ context.Context) (bool, error) {
				return false, errors.New("always failing")
			},
			expectedOutput: errors.New("reached retry limit"),
		},
		{
			name:          "terminal error stops immediately (stop=true with error)",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 10,
			inputFunc: func(_ context.Context) (bool, error) {
				return true, errors.New("fatal")
			},
			expectedOutput: errors.New("fatal"),
		},
		{
			name:          "stop=false with nil error exits cleanly",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 5,
			inputFunc: func(_ context.Context) (bool, error) {
				return false, nil
			},
			expectedOutput: nil,
		},
		{
			name: "pre-cancelled context exits without calling function many times",
			inputContext: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			inputInterval: time.Microsecond,
			inputAttempts: 100,
			inputFunc: func(_ context.Context) (bool, error) {
				return false, errors.New("would keep going")
			},
			// pre-cancelled → ctx.Err() fires before or after first call;
			// nil is acceptable when the fn was never invoked.
			expectedOutput: nil, // checked separately below via test.Error
		},
		{
			name:          "retry attempts = 1 exhausts on first failure",
			inputContext:  context.Background(),
			inputInterval: time.Microsecond,
			inputAttempts: 1,
			inputFunc: func(_ context.Context) (bool, error) {
				return false, errors.New("fail")
			},
			expectedOutput: errors.New("reached retry limit"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := retry(tc.inputContext, tc.inputInterval, tc.inputAttempts, tc.inputFunc)

			// pre-cancelled context test only requires an error (context.Canceled),
			// not an exact match.
			if tc.name == "pre-cancelled context exits without calling function many times" {
				test.Error(t, actualOutput)
				return
			}

			if tc.expectedOutput == nil {
				test.NoError(t, actualOutput)
			} else {
				test.Error(t, actualOutput)
				test.StrContains(t, actualOutput.Error(), tc.expectedOutput.Error())
			}
		})
	}
}

// Test_retry_cancelledMidwayIncludesLastError verifies that when the context
// is cancelled after a real error has been recorded, the returned error
// contains the last transient error message.
func Test_retry_cancelledMidwayIncludesLastError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var callCount int32

	err := retry(ctx, time.Microsecond, 100,
		func(_ context.Context) (bool, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 2 {
				cancel()
			}
			return false, errors.New("transient")
		})

	test.Error(t, err)
	test.StrContains(t, err.Error(), "transient")
}

// Test_retry_intervalIsRespected verifies that the sleep interval is actually
// observed between invocations.
func Test_retry_intervalIsRespected(t *testing.T) {
	const interval = 5 * time.Millisecond
	const attempts = 3

	start := time.Now()
	_ = retry(context.Background(), interval, attempts,
		func(_ context.Context) (bool, error) {
			return false, errors.New("keep retrying")
		})
	elapsed := time.Since(start)

	// attempts=3 → 2 sleeps of interval each before the limit is reached.
	minExpected := time.Duration(attempts-1) * interval
	if elapsed < minExpected {
		t.Errorf("elapsed %v < expected minimum %v — interval not respected", elapsed, minExpected)
	}
}
