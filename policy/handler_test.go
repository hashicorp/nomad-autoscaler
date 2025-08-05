// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

type MockLimiter struct {
	getSlotErr    error
	releaseCalled bool
}

func (m *MockLimiter) GetSlot(ctx context.Context, p *sdk.ScalingPolicy) error {
	return m.getSlotErr
}
func (m *MockLimiter) ReleaseSlot(p *sdk.ScalingPolicy) {
	m.releaseCalled = true
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
			name:           "resulting_cooldown_of_10_minutes",
		},
		{
			inputCooldown:  20 * time.Minute,
			inputTimestamp: baseTime,
			inputLastEvent: baseTime - 25*time.Minute.Nanoseconds(),
			expectedOutput: -5 * time.Minute,
			name:           "negative_cooldown_period;_ie._no_cooldown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := calculateRemainingCooldown(tc.inputCooldown, tc.inputTimestamp, tc.inputLastEvent)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func TestHandler_WaitAndScale(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Time{}
	}

	policy := &sdk.ScalingPolicy{
		ID:       "test-policy",
		Cooldown: 10 * time.Minute,
		Target: &sdk.ScalingPolicyTarget{
			Name:   "mock-target",
			Config: map[string]string{},
		},
	}

	tests := []struct {
		name          string
		getSlotErr    error
		scaleErr      error
		status        *sdk.TargetStatus
		expectedErr   error
		expectedState handlerState
		cooldownEnd   time.Time
	}{
		{
			name:          "success",
			expectedState: StateCooldown,
			cooldownEnd:   time.Time{}.Add(policy.Cooldown),
		},
		{
			name:          "slot_error",
			getSlotErr:    testErr,
			expectedState: StateWaitingTurn,
			expectedErr:   testErr,
		},
		{
			name:          "scale_error",
			scaleErr:      testErr,
			expectedState: StateScaling,
			expectedErr:   testErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter := &MockLimiter{
				getSlotErr: tc.getSlotErr,
			}

			target := &mockTargetController{
				err:    tc.scaleErr,
				status: tc.status,
			}

			handler := &Handler{
				log:              hclog.NewNullLogger(),
				policy:           policy,
				limiter:          limiter,
				targetController: target,
				state:            StateWaitingTurn,
				outOfCooldownOn:  time.Time{},
				nextAction:       sdk.ScalingAction{Count: 5},
			}

			err := handler.WaitAndScale(context.Background())
			must.True(t, errors.Is(err, tc.expectedErr))

			if tc.getSlotErr == nil {
				// Make sure we don't hold on to the slot.
				must.True(t, limiter.releaseCalled)
				must.True(t, target.scaleCalled)
			}

			must.Eq(t, tc.expectedState, handler.State())
			must.Eq(t, tc.cooldownEnd, handler.OutOfCooldownOn())
		})
	}
}
