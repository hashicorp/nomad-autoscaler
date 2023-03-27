// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScalingPolicy_Validate(t *testing.T) {
	testCases := []struct {
		name          string
		policy        *ScalingPolicy
		expectedError string
	}{
		{
			name:          "nil policy",
			policy:        nil,
			expectedError: "",
		},
		{
			name: "invalid on_check_error",
			policy: &ScalingPolicy{
				Type:         "horizontal",
				OnCheckError: "not-valid",
			},
			expectedError: "invalid value for on_check_error",
		},
		{
			name: "invalid on_error",
			policy: &ScalingPolicy{
				Type: "horizontal",
				Checks: []*ScalingPolicyCheck{
					{
						Name:    "invalid",
						OnError: "non-valid",
						Strategy: &ScalingPolicyStrategy{
							Name: "target-value",
						},
					},
					{
						Name:    "valid",
						OnError: "ignore",
						Strategy: &ScalingPolicyStrategy{
							Name: "target-value",
						},
					},
				},
			},
			expectedError: "invalid value for on_error in check",
		},
		{
			name: "DAS plugin with non-vertical policy",
			policy: &ScalingPolicy{
				Type: "horizontal",
				Checks: []*ScalingPolicyCheck{
					{
						Name: "invalid",
						Strategy: &ScalingPolicyStrategy{
							Name: "app-sizing-max",
						},
					},
					{
						Name: "valid",
						Strategy: &ScalingPolicyStrategy{
							Name: "target-value",
						},
					},
				},
			},
			expectedError: "can only be used with Dynamic Application Sizing",
		},
		{
			name: "valid policy",
			policy: &ScalingPolicy{
				Type:         "horizontal",
				OnCheckError: "ignore",
				Checks: []*ScalingPolicyCheck{
					{
						Name:    "valid",
						OnError: "fail",
						Strategy: &ScalingPolicyStrategy{
							Name: "target-value",
						},
					},
				},
			},
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.Validate()
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScalingPolicyTarget_IsNodePoolTarget(t *testing.T) {
	testCases := []struct {
		inputScalingPolicyTarget *ScalingPolicyTarget
		expectedOutput           bool
		name                     string
	}{
		{
			inputScalingPolicyTarget: &ScalingPolicyTarget{
				Config: map[string]string{"Job": "example", "Group": "cache"},
			},
			expectedOutput: false,
			name:           "job input target",
		},
		{
			inputScalingPolicyTarget: &ScalingPolicyTarget{
				Config: map[string]string{"node_class": "high-memory"},
			},
			expectedOutput: true,
			name:           "node_class input target",
		},
		{
			inputScalingPolicyTarget: &ScalingPolicyTarget{
				Config: map[string]string{"datacenter": "dc1"},
			},
			expectedOutput: true,
			name:           "datacenter input target",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputScalingPolicyTarget.IsNodePoolTarget(), tc.name)
		})
	}
}

func TestFileDecodePolicy_Translate(t *testing.T) {
	testCases := []struct {
		inputFileDecodePolicy *FileDecodeScalingPolicy
		expectedOutputPolicy  *ScalingPolicy
		name                  string
	}{
		{
			inputFileDecodePolicy: &FileDecodeScalingPolicy{
				Enabled: true,
				Min:     1,
				Max:     3,
				Doc: &FileDecodePolicyDoc{
					Cooldown:              10 * time.Millisecond,
					CooldownHCL:           "10ms",
					EvaluationInterval:    10 * time.Nanosecond,
					EvaluationIntervalHCL: "10ns",
					Checks: []*FileDecodePolicyCheckDoc{
						{
							Name:           "approach-speed",
							Source:         "front-sensor",
							Query:          "how-fast-am-i-going",
							QueryWindow:    time.Minute,
							QueryWindowHCL: "1m",
							Strategy: &ScalingPolicyStrategy{
								Name: "approach-velocity",
								Config: map[string]string{
									"target": "0.01ms",
								},
							},
						},
					},
					Target: &ScalingPolicyTarget{
						Name: "iss",
						Config: map[string]string{
							"docking-object": "forward-bulkhead",
						},
					},
				},
			},
			expectedOutputPolicy: &ScalingPolicy{
				ID:                 "",
				Min:                1,
				Max:                3,
				Enabled:            true,
				Cooldown:           10 * time.Millisecond,
				EvaluationInterval: 10 * time.Nanosecond,
				Checks: []*ScalingPolicyCheck{
					{
						Name:        "approach-speed",
						Source:      "front-sensor",
						Query:       "how-fast-am-i-going",
						QueryWindow: time.Minute,
						Strategy: &ScalingPolicyStrategy{
							Name: "approach-velocity",
							Config: map[string]string{
								"target": "0.01ms",
							},
						},
					},
				},
				Target: &ScalingPolicyTarget{
					Name: "iss",
					Config: map[string]string{
						"docking-object": "forward-bulkhead",
					},
				},
			},
			name: "fully hydrated decoded policy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.inputFileDecodePolicy.Translate()
			assert.Equal(t, tc.expectedOutputPolicy, got, tc.name)
		})
	}
}
