package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewScalingEvaluation(t *testing.T) {
	testCases := []struct {
		inputScalingPolicy *ScalingPolicy
		inputTargetStatus  *TargetStatus
		expectedOutput     *ScalingEvaluation
		name               string
	}{
		{
			inputScalingPolicy: &ScalingPolicy{
				ID:                 "test-test-test",
				Min:                2,
				Max:                40,
				Enabled:            true,
				Cooldown:           1 * time.Minute,
				EvaluationInterval: 1 * time.Second,
				Checks: []*ScalingPolicyCheck{
					{
						Name:   "first-check",
						Source: "apm-source",
						Query:  "some-gnarly-query-that-does-all-the-transformations",
						Strategy: &ScalingPolicyStrategy{
							Name:   "target-value",
							Config: map[string]string{"target": "10"},
						},
					},
					{
						Name:   "second-check",
						Source: "apm-source",
						Query:  "some-other-gnarly-query-that-does-all-the-transformations",
						Strategy: &ScalingPolicyStrategy{
							Name:   "target-value",
							Config: map[string]string{"target": "100"},
						},
					},
				},
				Target: &ScalingPolicyTarget{
					Name:   "astronomical-object",
					Config: map[string]string{"type": "galaxy", "object": "ngc-4649"},
				},
			},
			inputTargetStatus: &TargetStatus{
				Ready: true,
				Count: 57,
				Meta:  map[string]string{"r-shift": "0.003726"},
			},
			expectedOutput: &ScalingEvaluation{
				Policy: &ScalingPolicy{
					ID:                 "test-test-test",
					Min:                2,
					Max:                40,
					Enabled:            true,
					Cooldown:           1 * time.Minute,
					EvaluationInterval: 1 * time.Second,
					Checks: []*ScalingPolicyCheck{
						{
							Name:   "first-check",
							Source: "apm-source",
							Query:  "some-gnarly-query-that-does-all-the-transformations",
							Strategy: &ScalingPolicyStrategy{
								Name:   "target-value",
								Config: map[string]string{"target": "10"},
							},
						},
						{
							Name:   "second-check",
							Source: "apm-source",
							Query:  "some-other-gnarly-query-that-does-all-the-transformations",
							Strategy: &ScalingPolicyStrategy{
								Name:   "target-value",
								Config: map[string]string{"target": "100"},
							},
						},
					},
					Target: &ScalingPolicyTarget{
						Name:   "astronomical-object",
						Config: map[string]string{"type": "galaxy", "object": "ngc-4649"},
					},
				},
				TargetStatus: &TargetStatus{
					Ready: true,
					Count: 57,
					Meta:  map[string]string{"r-shift": "0.003726"},
				},
				CheckEvaluations: []*ScalingCheckEvaluation{
					{
						Check: &ScalingPolicyCheck{
							Name:   "first-check",
							Source: "apm-source",
							Query:  "some-gnarly-query-that-does-all-the-transformations",
							Strategy: &ScalingPolicyStrategy{
								Name:   "target-value",
								Config: map[string]string{"target": "10"},
							},
						},
						Metrics: nil,
						Action: &ScalingAction{
							Meta: make(map[string]interface{}),
						},
					},
					{
						Check: &ScalingPolicyCheck{
							Name:   "second-check",
							Source: "apm-source",
							Query:  "some-other-gnarly-query-that-does-all-the-transformations",
							Strategy: &ScalingPolicyStrategy{
								Name:   "target-value",
								Config: map[string]string{"target": "100"},
							},
						},
						Metrics: nil,
						Action: &ScalingAction{
							Meta: make(map[string]interface{}),
						},
					},
				},
			},
			name: "basic struct population check",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := NewScalingEvaluation(tc.inputScalingPolicy, tc.inputTargetStatus)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
