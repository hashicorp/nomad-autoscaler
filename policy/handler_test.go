package policy

import (
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

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
		{
			inputCooldown:  20 * time.Minute,
			inputTimestamp: baseTime,
			inputLastEvent: baseTime - 25*time.Minute.Nanoseconds(),
			expectedOutput: -5 * time.Minute,
			name:           "negative cooldown period; ie. no cooldown",
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
