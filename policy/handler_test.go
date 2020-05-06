package policy

import (
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestHandler_isInCooldown(t *testing.T) {
	testCases := []struct {
		inputCooldown  time.Duration
		inputTimestamp int64
		inputLastEvent uint64
		expectedOutput bool
		name           string
	}{
		{
			inputCooldown:  100 * time.Hour,
			inputTimestamp: time.Date(2020, time.April, 1, 9, 10, 0, 0, time.UTC).UnixNano(),
			inputLastEvent: uint64(time.Date(2020, time.April, 1, 9, 9, 0, 0, time.UTC).UnixNano()),
			expectedOutput: true,
			name:           "policy is in current cooldown",
		},
		{
			inputCooldown:  1 * time.Hour,
			inputTimestamp: time.Date(2020, time.April, 1, 9, 10, 0, 0, time.UTC).UnixNano(),
			inputLastEvent: uint64(time.Date(2020, time.April, 1, 8, 9, 0, 0, time.UTC).UnixNano()),
			expectedOutput: false,
			name:           "policy should not be in cooldown",
		},
	}

	h := NewHandler("", hclog.NewNullLogger(), nil, nil)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := h.isInCooldown(tc.inputCooldown, tc.inputTimestamp, tc.inputLastEvent)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func TestHandler_calculateRemainingCooldown(t *testing.T) {

	baseTime := time.Now().UTC().UnixNano()

	testCases := []struct {
		inputCooldown  time.Duration
		inputTimestamp int64
		inputLastEvent int64
		expectedOutput time.Duration
		name           string
	}{
		{
			inputCooldown:  20 * time.Minute,
			inputTimestamp: baseTime,
			inputLastEvent: baseTime - 10*time.Minute.Nanoseconds(),
			expectedOutput: 10 * time.Minute,
			name:           "resulting cooldown of 10 minutes",
		},
	}

	h := NewHandler("", hclog.NewNullLogger(), nil, nil)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := h.calculateRemainingCooldown(tc.inputCooldown, tc.inputTimestamp, tc.inputLastEvent)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
