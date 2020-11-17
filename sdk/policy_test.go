package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
