// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func Test_decodeFile(t *testing.T) {
	testCases := []struct {
		inputFile              string
		expectedOutputPolicies map[string]*sdk.ScalingPolicy
		expectedOutputError    error
		name                   string
	}{
		{
			inputFile: "./test-fixtures/full-cluster-policy.hcl",
			expectedOutputPolicies: map[string]*sdk.ScalingPolicy{
				"full-cluster-policy": {
					ID:                 "",
					Type:               sdk.ScalingPolicyTypeCluster,
					Enabled:            true,
					Min:                10,
					Max:                100,
					Cooldown:           10 * time.Minute,
					EvaluationInterval: 1 * time.Minute,
					OnCheckError:       "error",
					Checks: []*sdk.ScalingPolicyCheck{
						{
							Name:              "cpu_nomad",
							Group:             "cpu",
							Source:            "nomad_apm",
							Query:             "cpu_high-memory",
							QueryWindow:       time.Minute,
							QueryWindowOffset: 2 * time.Minute,
							Strategy: &sdk.ScalingPolicyStrategy{
								Name: "target-value",
								Config: map[string]string{
									"target": "80",
								},
							},
						},
						{
							Name:    "memory_prom",
							OnError: "ignore",
							Source:  "prometheus",
							Query:   "nomad_client_allocated_memory*100/(nomad_client_allocated_memory+nomad_client_unallocated_memory)",
							Strategy: &sdk.ScalingPolicyStrategy{
								Name: "target-value",
								Config: map[string]string{
									"target": "80",
								},
							},
						},
					},
					Target: &sdk.ScalingPolicyTarget{
						Name: "aws-asg",
						Config: map[string]string{
							"aws_asg_name":        "my-target-asg",
							"node_class":          "high-memory",
							"node_drain_deadline": "15m",
						},
					},
				},
			},
			expectedOutputError: nil,
			name:                "full parsable cluster scaling policy",
		},
		{
			inputFile: "./test-fixtures/full-task-group-policy.hcl",
			expectedOutputPolicies: map[string]*sdk.ScalingPolicy{
				"full-task-group-policy": {
					ID:                 "",
					Type:               sdk.ScalingPolicyTypeHorizontal,
					Enabled:            true,
					Min:                1,
					Max:                10,
					Cooldown:           1 * time.Minute,
					EvaluationInterval: 30 * time.Second,
					Checks: []*sdk.ScalingPolicyCheck{
						{
							Name:   "cpu_nomad",
							Source: "nomad_apm",
							Query:  "avg_cpu",
							Strategy: &sdk.ScalingPolicyStrategy{
								Name: "target-value",
								Config: map[string]string{
									"target": "80",
								},
							},
						},
						{
							Name:   "memory_nomad",
							Source: "nomad_apm",
							Query:  "avg_memory",
							Strategy: &sdk.ScalingPolicyStrategy{
								Name: "target-value",
								Config: map[string]string{
									"target": "80",
								},
							},
						},
					},
					Target: &sdk.ScalingPolicyTarget{
						Name: "nomad",
						Config: map[string]string{
							"Group": "cache",
							"Job":   "example",
						},
					},
				},
			},
			expectedOutputError: nil,
			name:                "full parsable task group scaling policy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, actualError := decodeFile(tc.inputFile)
			assert.Equal(t, tc.expectedOutputPolicies, got, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)

			// Print unexpected errors.
			if actualError != nil && tc.expectedOutputError == nil {
				t.Logf("%s", actualError)
			}
		})
	}
}

func Test_decodePolicyDoc_QueryWindowInstant(t *testing.T) {
	decodePolicy := &sdk.FileDecodeScalingPolicy{
		Doc: &sdk.FileDecodePolicyDoc{
			Checks: []*sdk.FileDecodePolicyCheckDoc{{
				QueryWindowHCL:       "instant",
				QueryWindowOffsetHCL: "10m",
			}},
		},
	}

	err := decodePolicyDoc(decodePolicy)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), decodePolicy.Doc.Checks[0].QueryWindow)
	assert.Equal(t, 10*time.Minute, decodePolicy.Doc.Checks[0].QueryWindowOffset)

	translated := &sdk.ScalingPolicyCheck{}
	decodePolicy.Doc.Checks[0].Translate(translated)
	assert.True(t, translated.QueryInstant)
	assert.Equal(t, time.Duration(0), translated.QueryWindow)
	assert.Equal(t, 10*time.Minute, translated.QueryWindowOffset)
}

func Test_decodePolicyDoc_QueryWindowDuration(t *testing.T) {
	decodePolicy := &sdk.FileDecodeScalingPolicy{
		Doc: &sdk.FileDecodePolicyDoc{
			Checks: []*sdk.FileDecodePolicyCheckDoc{{
				QueryWindowHCL:       "5m",
				QueryWindowOffsetHCL: "10m",
			}},
		},
	}

	err := decodePolicyDoc(decodePolicy)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, decodePolicy.Doc.Checks[0].QueryWindow)
	assert.Equal(t, 10*time.Minute, decodePolicy.Doc.Checks[0].QueryWindowOffset)

	translated := &sdk.ScalingPolicyCheck{}
	decodePolicy.Doc.Checks[0].Translate(translated)
	assert.False(t, translated.QueryInstant)
	assert.Equal(t, 5*time.Minute, translated.QueryWindow)
	assert.Equal(t, 10*time.Minute, translated.QueryWindowOffset)
}

func Test_decodePolicyDoc_QueryWindowInvalid(t *testing.T) {
	decodePolicy := &sdk.FileDecodeScalingPolicy{
		Doc: &sdk.FileDecodePolicyDoc{
			Checks: []*sdk.FileDecodePolicyCheckDoc{{
				QueryWindowHCL: "not-a-duration",
			}},
		},
	}

	err := decodePolicyDoc(decodePolicy)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not-a-duration"))
}
