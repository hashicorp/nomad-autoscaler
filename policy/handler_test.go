// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

type MockLimiter struct {
	getSlotErr error

	callslock     sync.Mutex
	releaseCalled bool
}

func (m *MockLimiter) getReleaseCalled() bool {
	m.callslock.Lock()
	defer m.callslock.Unlock()

	return m.releaseCalled
}

func (m *MockLimiter) GetSlot(ctx context.Context, p *sdk.ScalingPolicy) error {
	return m.getSlotErr
}
func (m *MockLimiter) ReleaseSlot(p *sdk.ScalingPolicy) {
	m.callslock.Lock()
	defer m.callslock.Unlock()

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

			err := handler.waitAndScale(context.Background())
			must.True(t, errors.Is(err, tc.expectedErr))

			if tc.getSlotErr == nil {
				// Make sure we don't hold on to the slot.
				must.True(t, limiter.releaseCalled)
				must.True(t, target.getScaleCalled())
			}

			must.Eq(t, tc.expectedState, handler.getState())
			must.Eq(t, tc.cooldownEnd, handler.getOutOfCooldownOn())
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
		{
			name: "groupWinner_handler_is_nil",
			checkGroups: map[string][]checkResult{
				"group1": {
					{action: actionUp, handler: nil, group: "group1"},   // handler is nil
					{action: actionNone, handler: nil, group: "group1"}, // handler is nil
				},
				"group2": {
					{action: actionUp, handler: runnerA, group: "group2"},
				},
			},
			wantAction:  actionUp,
			wantHandler: runnerA,
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

func TestHandler_Run_TargetError_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)
	updatesCh := make(chan *sdk.ScalingPolicy)
	policy := &sdk.ScalingPolicy{
		ID:                 "test-policy",
		EvaluationInterval: 10 * time.Millisecond,
		Target:             &sdk.ScalingPolicyTarget{Name: "mock-target", Config: map[string]string{}},
	}

	handler := &Handler{
		log:              hclog.NewNullLogger(),
		policy:           policy,
		updatesCh:        updatesCh,
		errChn:           errCh,
		targetController: &mockTargetController{statusErr: testErr},
		state:            StateIdle,
	}

	go handler.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		must.True(t, errors.Is(err, testErr))
	default:
		t.Fatal("expected error getting target status")
	}

	must.Eq(t, StateIdle, handler.getState())
	must.Eq(t, time.Time{}, handler.getOutOfCooldownOn())
	must.Eq(t, sdk.ScalingAction{}, handler.getNextAction())
}
func TestHandler_Run_TargetNotFound_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	updatesCh := make(chan *sdk.ScalingPolicy)
	policy := &sdk.ScalingPolicy{
		ID:                 "test-policy",
		EvaluationInterval: 10 * time.Millisecond,
		Target:             &sdk.ScalingPolicyTarget{Name: "mock-target", Config: map[string]string{}},
	}

	handler := &Handler{
		log:              hclog.NewNullLogger(),
		policy:           policy,
		updatesCh:        updatesCh,
		errChn:           errCh,
		targetController: &mockTargetController{status: nil},
		state:            StateIdle,
	}

	go handler.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		must.True(t, errors.Is(err, errTargetNotFound))
	default:
		t.Fatal("expected error for target not found")
	}

	must.Eq(t, StateIdle, handler.getState())
	must.Eq(t, time.Time{}, handler.getOutOfCooldownOn())
	must.Eq(t, sdk.ScalingAction{}, handler.getNextAction())
}

func TestHandler_Run_TargetNotReady_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	updatesCh := make(chan *sdk.ScalingPolicy)
	policy := &sdk.ScalingPolicy{
		ID:                 "test-policy",
		EvaluationInterval: 10 * time.Millisecond,
		Target:             &sdk.ScalingPolicyTarget{Name: "mock-target", Config: map[string]string{}},
	}

	handler := &Handler{
		log:              hclog.NewNullLogger(),
		policy:           policy,
		updatesCh:        updatesCh,
		errChn:           errCh,
		targetController: &mockTargetController{status: &sdk.TargetStatus{Ready: false, Count: 1, Meta: map[string]string{}}},
		state:            StateIdle,
	}

	go handler.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()

	must.Eq(t, StateIdle, handler.getState())
	must.Eq(t, time.Time{}, handler.getOutOfCooldownOn())
	must.Eq(t, sdk.ScalingAction{}, handler.getNextAction())
}

var policy = &sdk.ScalingPolicy{
	ID:                 "test-policy",
	EvaluationInterval: 20 * time.Millisecond,
	Min:                1,
	Max:                10,
	Cooldown:           10 * time.Minute,
	CooldownOnScaleUp:  5 * time.Minute,
	Target: &sdk.ScalingPolicyTarget{
		Name:   "mock-target",
		Config: map[string]string{},
	},
	Checks: []*sdk.ScalingPolicyCheck{
		{
			Name:   "mock-check",
			Source: "mock-source",
			Query:  "mock-query",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "mock-strategy",
			},
		},
	},
}

func TestHandler_Run_ScalingNotNeeded_Integration(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Time{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	updatesCh := make(chan *sdk.ScalingPolicy)

	mapml := &mockAPMLooker{
		t:         t,
		query:     "mock-query",
		timeRange: sdk.TimeRange{From: time.Time{}, To: time.Time{}},
		metrics: sdk.TimestampedMetrics{
			{Timestamp: nowFunc().Add(-time.Minute), Value: 1.0},
			{Timestamp: nowFunc(), Value: 2.0},
		},
	}

	mtc := &mockTargetController{
		status: &sdk.TargetStatus{
			Ready: true,
			Count: 1,
			Meta:  map[string]string{},
		},
	}

	mdg := &MockDependencyGetter{
		APMLooker: mapml,
		StrategyRunner: &mockStrategyRunner{
			t: t,
		},
	}

	handler := &Handler{
		log:              hclog.NewNullLogger(),
		policy:           policy,
		updatesCh:        updatesCh,
		errChn:           errCh,
		targetController: mtc,
		state:            StateIdle,
		pm:               mdg,
	}

	must.NoError(t, handler.loadCheckRunners())

	go handler.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()

	must.False(t, mtc.getScaleCalled())
	must.Eq(t, StateIdle, handler.getState())
	must.Eq(t, time.Time{}, handler.getOutOfCooldownOn())
	must.Eq(t, sdk.ScalingAction{}, handler.getNextAction())
}

func TestHandler_Run_ScalingNeededAndCooldown_Integration(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Time{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	updatesCh := make(chan *sdk.ScalingPolicy)

	mapml := &mockAPMLooker{
		t:         t,
		query:     "mock-query",
		timeRange: sdk.TimeRange{From: time.Time{}, To: time.Time{}},
		metrics: sdk.TimestampedMetrics{
			{Timestamp: nowFunc().Add(-time.Minute), Value: 1.0},
			{Timestamp: nowFunc(), Value: 2.0},
		},
	}

	mtc := &mockTargetController{
		status: &sdk.TargetStatus{
			Ready: true,
			Count: 5,
			Meta:  map[string]string{},
		},
	}

	mdg := &MockDependencyGetter{
		APMLooker: mapml,
		StrategyRunner: &mockStrategyRunner{
			t:         t,
			count:     10,
			direction: sdk.ScaleDirectionUp,
		},
	}

	ml := &MockLimiter{}

	handler := &Handler{
		log:              hclog.NewNullLogger(),
		policy:           policy,
		updatesCh:        updatesCh,
		errChn:           errCh,
		targetController: mtc,
		state:            StateIdle,
		pm:               mdg,
		limiter:          ml,
	}

	must.NoError(t, handler.loadCheckRunners())

	go handler.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()

	must.True(t, mtc.getScaleCalled())
	must.Eq(t, StateCooldown, handler.getState())
	must.True(t, ml.getReleaseCalled())
	must.Eq(t, time.Time{}.Add(5*time.Minute), handler.getOutOfCooldownOn())
	must.Eq(t, sdk.ScalingAction{
		Count:     10,
		Direction: sdk.ScaleDirectionUp,
		Meta: map[string]interface{}{
			"nomad_policy_id": policy.ID,
		},
	}, handler.getNextAction())
}
