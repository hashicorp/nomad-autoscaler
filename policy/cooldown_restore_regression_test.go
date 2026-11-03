// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func restoredCooldownHandler(t *testing.T, cooldown, cooldownOnScaleUp time.Duration, meta map[string]string) (*Handler, *mockTargetController) {
	t.Helper()
	previousNow := nowFunc
	nowFunc = time.Now
	t.Cleanup(func() { nowFunc = previousNow })
	target := &mockTargetController{status: &sdk.TargetStatus{Count: 2, Ready: true, Meta: meta}}
	handler, err := NewPolicyHandler(HandlerConfig{
		Log: hclog.NewNullLogger(),
		Policy: &sdk.ScalingPolicy{
			ID:                 "cooldown-restore-regression",
			Type:               sdk.ScalingPolicyTypeCluster,
			Min:                1,
			Max:                5,
			EvaluationInterval: time.Second,
			Cooldown:           cooldown,
			CooldownOnScaleUp:  cooldownOnScaleUp,
			Target:             &sdk.ScalingPolicyTarget{Name: "mock-target", Config: map[string]string{}},
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
		name                                 string
		cooldown, cooldownOnScaleUp, elapsed time.Duration
	}{
		{"short", 6 * time.Second, 6 * time.Second, 2 * time.Second},
		{"five_minutes", 5 * time.Minute, 5 * time.Minute, time.Minute},
		{"expired", 6 * time.Second, 6 * time.Second, 8 * time.Second},
		{"future_event", 6 * time.Second, 6 * time.Second, -2 * time.Second},
		{"shorter_scale_up", time.Minute, 6 * time.Second, 2 * time.Second},
		{"longer_scale_up", 6 * time.Second, time.Minute, 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := now.Add(-tc.elapsed)
			handler, _ := restoredCooldownHandler(t, tc.cooldown, tc.cooldownOnScaleUp,
				map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(last.UnixNano(), 10)})
			want := last.Add(tc.cooldownOnScaleUp)
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
			handler, _ := restoredCooldownHandler(t, time.Minute, time.Second, tc.meta)
			if handler.getState() != StateIdle || !handler.getOutOfCooldownOn().IsZero() {
				t.Fatal("nonpositive/missing/malformed last event unexpectedly restored cooldown")
			}
		})
	}
}

func TestNewPolicyHandlerRestoredCooldownGatesRun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
	}{
		{"remaining", time.Second},
		{"already_expired", 6 * time.Second},
		{"future_event", -2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const cooldownOnScaleUp = 5 * time.Second
				last := time.Now().Add(-tc.elapsed)
				deadline := last.Add(cooldownOnScaleUp)
				handler, target := restoredCooldownHandler(t, time.Minute, cooldownOnScaleUp,
					map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(last.UnixNano(), 10)})
				check := &stubChecker{action: sdk.ScalingAction{Count: 3, Direction: sdk.ScaleDirectionUp}}
				handler.checkRunners = []checker{check}
				handler.limiter = &MockLimiter{}
				ctx, cancel := context.WithCancel(context.Background())
				defer func() {
					cancel()
					synctest.Wait()
				}()
				go handler.Run(ctx)
				synctest.Wait()

				// Advance the fake clock to the first evaluation, then wait for
				// the handler and any scaling goroutine to finish their work.
				time.Sleep(handler.policy.EvaluationInterval)
				synctest.Wait()
				if check.callCount() == 0 {
					t.Fatal("no evaluations observed")
				}
				if !time.Now().After(deadline) {
					if target.getScaleCalled() || handler.getState() != StateCooldown {
						t.Fatal("restored cooldown did not suppress scaling before its deadline")
					}

					time.Sleep(time.Until(deadline))
					synctest.Wait()
					if target.getScaleCalled() {
						t.Fatal("restored cooldown allowed Scale at its deadline")
					}

					time.Sleep(handler.policy.EvaluationInterval)
					synctest.Wait()
				}
				if !target.getScaleCalled() {
					t.Fatal("restored cooldown did not allow Scale after its deadline")
				}
				if handler.getState() != StateCooldown {
					t.Fatalf("completed scale did not start a new cooldown: state=%v", handler.getState())
				}
				if got, want := handler.getOutOfCooldownOn(), time.Now().Add(cooldownOnScaleUp); !got.Equal(want) {
					t.Fatalf("new cooldown deadline=%s want=%s", got, want)
				}
			})
		})
	}
}

func TestNewPolicyHandlerHasNoStaleRestoreTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		last := time.Now().Add(-120 * time.Millisecond)
		handler, _ := restoredCooldownHandler(t, time.Second, 200*time.Millisecond,
			map[string]string{sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(last.UnixNano(), 10)})
		handler.updateState(StateScaling)
		time.Sleep(time.Second)
		synctest.Wait()
		if handler.getState() != StateScaling {
			t.Fatalf("constructor timer reset a newer state: %v", handler.getState())
		}
	})
}
