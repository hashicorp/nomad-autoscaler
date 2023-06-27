// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewScalingEvaluation(t *testing.T) {
	testCases := []struct {
		inputScalingPolicy *ScalingPolicy
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
							Meta: map[string]interface{}{
								"nomad_policy_id": "test-test-test",
							},
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
							Meta: map[string]interface{}{
								"nomad_policy_id": "test-test-test",
							},
						},
					},
				},
			},
			name: "basic struct population check",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := NewScalingEvaluation(tc.inputScalingPolicy)
			assert.NotEmpty(t, actualOutput.ID)
			assert.NotZero(t, actualOutput.CreateTime)
			// Fill in randomly generated values
			tc.expectedOutput.ID = actualOutput.ID
			tc.expectedOutput.CreateTime = actualOutput.CreateTime
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
