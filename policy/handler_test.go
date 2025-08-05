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
				scaleErr: tc.scaleErr,
				status:   tc.status,
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

func Test_pickWinnerActionFromGroups(t *testing.T) {

	actionNone := &sdk.ScalingAction{
		Direction: sdk.ScaleDirectionNone,
		Count:     0,
	}
	actionUp := &sdk.ScalingAction{
		Direction: sdk.ScaleDirectionUp,
		Count:     5,
	}
	actionDown := &sdk.ScalingAction{
		Direction: sdk.ScaleDirectionDown,
		Count:     1,
	}

	runnerA := &checkRunner{
		check: &sdk.ScalingPolicyCheck{Name: "A"},
	}
	runnerB := &checkRunner{
		check: &sdk.ScalingPolicyCheck{Name: "B"},
	}
	runnerC := &checkRunner{
		check: &sdk.ScalingPolicyCheck{Name: "C"},
	}

	tests := []struct {
		name        string
		checkGroups map[string][]checkResult
		wantAction  *sdk.ScalingAction
		wantHandler *checkRunner
	}{
		{
			name: "all_none_in_group",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionNone, handler: runnerA, group: "group1"},
					{action: actionNone, handler: runnerB, group: "group1"},
				},
			},
			wantAction:  actionNone,
			wantHandler: runnerA,
		},
		{
			name: "up_beats_none",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionNone, handler: runnerA, group: "group1"},
					{action: actionUp, handler: runnerB, group: "group1"},
				},
			},
			wantAction:  actionUp,
			wantHandler: runnerB,
		},
		{
			name: "down_beats_none",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionNone, handler: runnerA, group: "group1"},
					{action: actionDown, handler: runnerB, group: "group1"},
				},
			},
			wantAction:  actionDown,
			wantHandler: runnerB,
		},
		{
			name: "up_beats_down",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionDown, handler: runnerA, group: "group1"},
					{action: actionUp, handler: runnerB, group: "group1"},
				},
			},
			wantAction:  actionUp,
			wantHandler: runnerB,
		},
		{
			name: "multiple_groups_up_wins",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionNone, handler: runnerA, group: "group1"},
				},
				"group2": {
					{action: actionUp, handler: runnerB, group: "group2"},
					{action: actionDown, handler: runnerC, group: "group2"},
				},
			},
			wantAction:  actionUp,
			wantHandler: runnerB,
		},
		{
			name:        "empty_input",
			checkGroups: map[string][]checkResult{},
			wantAction:  nil,
			wantHandler: nil,
		},
		{
			name: "nil_actions_ignored",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: nil, handler: runnerA, group: "group1"},
					{action: actionUp, handler: runnerB, group: "group1"},
				},
			},
			wantAction:  actionUp,
			wantHandler: runnerB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pickWinnerActionFromGroups(tt.checkGroups)

			if tt.wantAction == nil {
				must.Nil(t, result.action)
				must.Nil(t, result.handler)
			} else {
				must.NotNil(t, result.action)
				must.Eq(t, tt.wantAction.Direction, result.action.Direction)
				must.Eq(t, tt.wantAction.Count, result.action.Count)
				must.NotNil(t, result.handler)
				must.Eq(t, tt.wantHandler.check.Name, result.handler.check.Name)
			}
		})
	}
}
