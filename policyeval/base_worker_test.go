package policyeval

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

type mockTargetScaler struct {
	count     int64
	direction sdk.ScaleDirection
}

func (mts *mockTargetScaler) Scale(action sdk.ScalingAction, config map[string]string) error {
	mts.setCount(action.Count)
	return nil
}

func (mts *mockTargetScaler) setCount(newCount int64) {
	mts.count = newCount
}

func (mts *mockTargetScaler) getCount() int64 {
	return mts.count
}

type mockCoolDownEnforcer struct {
	policyID string
	duration time.Duration
}

func (mce *mockCoolDownEnforcer) EnforceCooldown(_ string, coolDownDuration time.Duration) {
	mce.duration = coolDownDuration
}

func TestScaleTarget_Cooldown(t *testing.T) {

	testPolicy := &sdk.ScalingPolicy{
		ID:                "test_policy",
		Cooldown:          10 * time.Minute,
		CooldownOnScaleUp: 20 * time.Second,
	}

	testStatus := &sdk.TargetStatus{
		Count: 2,
	}

	testCases := []struct {
		name             string
		scalingAction    sdk.ScalingAction
		target           *sdk.ScalingPolicyTarget
		expectedCooldown time.Duration
		expectedCount    int64
	}{
		{
			name: "scaling_up",

			scalingAction: sdk.ScalingAction{
				Count:     4,
				Direction: sdk.ScaleDirectionUp,
			},
			target: &sdk.ScalingPolicyTarget{
				Name: "testTarget",
			},
			expectedCooldown: testPolicy.CooldownOnScaleUp,
			expectedCount:    4,
		},
		{
			name: "scaling_down",

			scalingAction: sdk.ScalingAction{
				Count:     1,
				Direction: sdk.ScaleDirectionDown,
			},
			target: &sdk.ScalingPolicyTarget{
				Name: "testTarget",
			},
			expectedCooldown: testPolicy.Cooldown,
			expectedCount:    1,
		},
		{
			name: "scaling_dry_run",
			scalingAction: sdk.ScalingAction{
				Count:     4,
				Direction: sdk.ScaleDirectionUp,
				Meta:      map[string]interface{}{},
			},
			target: &sdk.ScalingPolicyTarget{
				Name: "testTarget",
				Config: map[string]string{
					"dry-run": "true",
				},
			},
			expectedCooldown: testPolicy.Cooldown,
			expectedCount:    -1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testPolicy.Target = tc.target
			mcde := &mockCoolDownEnforcer{}
			mts := &mockTargetScaler{}

			testBaseWorker := BaseWorker{
				logger:           hclog.NewNullLogger(),
				cooldownEnforcer: mcde,
			}

			testBaseWorker.scaleTarget(mts, testPolicy, tc.scalingAction, testStatus)

			must.Eq(t, tc.expectedCooldown, mcde.duration)
			must.Eq(t, tc.expectedCount, mts.getCount())
		})
	}

}
