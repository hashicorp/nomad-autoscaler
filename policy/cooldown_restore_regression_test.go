// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func restoredCooldownHandler(t *testing.T, now time.Time, cooldown time.Duration, meta map[string]string) (*Handler, *mockTargetController) {
	t.Helper()
	previousNow := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = previousNow })
	target := &mockTargetController{status: &sdk.TargetStatus{Count: 2, Ready: true, Meta: meta}}
	handler, err := NewPolicyHandler(HandlerConfig{
		Log: hclog.NewNullLogger(),
		Policy: &sdk.ScalingPolicy{
			ID: "cooldown-restore-regression", Type: sdk.ScalingPolicyTypeCluster,
			Min: 1, Max: 5, EvaluationInterval: 2 * time.Millisecond,
			Cooldown: cooldown, CooldownOnScaleUp: cooldown,
			Target: &sdk.ScalingPolicyTarget{Name: "mock-target", Config: map[string]string{}},
		},
		TargetController: target,
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler, target
}

func TestNewPolicyHandlerRestoresCooldownDeadline(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, tc := range []struct {
		name              string
		cooldown, elapsed time.Duration
	}{
		{"short", 6 * time.Second, 2 * time.Second},
		{"five_minutes", 5 * time.Minute, time.Minute},
		{"expired", 6 * time.Second, 8 * time.Second},
		{"future_event", 6 * time.Second, -2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := now.Add(-tc.elapsed)
			handler, _ := restoredCooldownHandler(t, now, tc.cooldown,
				map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(last.UnixNano(), 10)})
			want := last.Add(tc.cooldown)
			if got := handler.getOutOfCooldownOn(); !got.Equal(want) {
				t.Fatalf("restored cooldown deadline=%s want=%s", got, want)
			}
		})
	}
}

func TestNewPolicyHandlerWithoutPositiveLastEventStaysIdle(t *testing.T) {
	for _, tc := range []struct {
		name string
		meta map[string]string
	}{
		{"missing", nil},
		{"malformed", map[string]string{sdk.TargetStatusMetaKeyLastEvent: "not-a-timestamp"}},
		{"zero", map[string]string{sdk.TargetStatusMetaKeyLastEvent: "0"}},
		{"negative", map[string]string{sdk.TargetStatusMetaKeyLastEvent: "-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, _ := restoredCooldownHandler(t, time.Now().UTC(), time.Second, tc.meta)
			if handler.getState() != StateIdle || !handler.getOutOfCooldownOn().IsZero() {
				t.Fatal("nonpositive/missing/malformed last event unexpectedly restored cooldown")
			}
		})
	}
}

func TestNewPolicyHandlerRestoredCooldownGatesRun(t *testing.T) {
	now := time.Now().UTC()
	handler, target := restoredCooldownHandler(t, now, time.Minute,
		map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(now.Add(-time.Second).UnixNano(), 10)})
	check := &stubChecker{action: sdk.ScalingAction{Count: 3, Direction: sdk.ScaleDirectionUp}}
	handler.checkRunners = []checker{check}
	handler.limiter = &MockLimiter{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); handler.Run(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not stop")
	}
	if check.calls == 0 {
		t.Fatal("no evaluations observed")
	}
	if target.getScaleCalled() {
		t.Fatal("restored cooldown allowed Scale before its deadline")
	}
	if handler.getState() != StateCooldown {
		t.Fatalf("restored cooldown state=%v", handler.getState())
	}
}

func TestNewPolicyHandlerHasNoStaleRestoreTimer(t *testing.T) {
	now := time.Now().UTC()
	handler, _ := restoredCooldownHandler(t, now, 200*time.Millisecond,
		map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(now.Add(-120*time.Millisecond).UnixNano(), 10)})
	handler.updateState(StateScaling)
	time.Sleep(200 * time.Millisecond)
	if handler.getState() != StateScaling {
		t.Fatalf("constructor timer reset a newer state: %v", handler.getState())
	}
}
