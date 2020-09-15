package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileDecodePolicy_Translate(t *testing.T) {
	testCases := []struct {
		inputFileDecodePolicy *FileDecodeScalingPolicy
		inputPolicy           *ScalingPolicy
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
					Checks: []*ScalingPolicyCheck{
						{
							Name:   "approach-speed",
							Source: "front-sensor",
							Query:  "how-fast-am-i-going",
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
			inputPolicy: &ScalingPolicy{},
			expectedOutputPolicy: &ScalingPolicy{
				ID:                 "",
				Min:                1,
				Max:                3,
				Enabled:            true,
				Cooldown:           10 * time.Millisecond,
				EvaluationInterval: 10 * time.Nanosecond,
				Checks: []*ScalingPolicyCheck{
					{
						Name:   "approach-speed",
						Source: "front-sensor",
						Query:  "how-fast-am-i-going",
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
			tc.inputFileDecodePolicy.Translate(tc.inputPolicy)
			assert.Equal(t, tc.expectedOutputPolicy, tc.inputPolicy, tc.name)
		})
	}
}
