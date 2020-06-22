package policy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPolicy_ApplyDefaults(t *testing.T) {
	testCases := []struct {
		inputPolicy          *Policy
		inputDefaults        *ConfigDefaults
		expectedOutputPolicy *Policy
		name                 string
	}{
		{
			inputPolicy: &Policy{
				Cooldown: 20 * time.Second,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &Policy{
				Cooldown:           20 * time.Second,
				EvaluationInterval: 5 * time.Second,
			},
			name: "evaluation interval set to default",
		},
		{
			inputPolicy: &Policy{
				EvaluationInterval: 15 * time.Second,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           11 * time.Second,
			},
			expectedOutputPolicy: &Policy{
				Cooldown:           11 * time.Second,
				EvaluationInterval: 15 * time.Second,
			},
			name: "cooldown set to default",
		},
		{
			inputPolicy: &Policy{},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &Policy{
				Cooldown:           10 * time.Second,
				EvaluationInterval: 5 * time.Second,
			},
			name: "evaluation interval and cooldown set to default",
		},
		{
			inputPolicy: &Policy{
				Cooldown:           10 * time.Minute,
				EvaluationInterval: 5 * time.Minute,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &Policy{
				Cooldown:           10 * time.Minute,
				EvaluationInterval: 5 * time.Minute,
			},
			name: "neither set to default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputPolicy.ApplyDefaults(tc.inputDefaults)
			assert.Equal(t, tc.expectedOutputPolicy, tc.inputPolicy, tc.name)
		})
	}
}

func TestFileDecodePolicy_Translate(t *testing.T) {
	testCases := []struct {
		inputFileDecodePolicy *FileDecodePolicy
		inputPolicy           *Policy
		expectedOutputPolicy  *Policy
		name                  string
	}{
		{
			inputFileDecodePolicy: &FileDecodePolicy{
				Enabled: true,
				Min:     1,
				Max:     3,
				Doc: &FileDecodePolicyDoc{
					Cooldown:              10 * time.Millisecond,
					CooldownHCL:           "10ms",
					EvaluationInterval:    10 * time.Nanosecond,
					EvaluationIntervalHCL: "10ns",
					Checks: []*Check{
						{
							Name:   "approach-speed",
							Source: "front-sensor",
							Query:  "how-fast-am-i-going",
							Strategy: &Strategy{
								Name: "approach-velocity",
								Config: map[string]string{
									"target": "0.01ms",
								},
							},
						},
					},
					Target: &Target{
						Name: "iss",
						Config: map[string]string{
							"docking-object": "forward-bulkhead",
						},
					},
				},
			},
			inputPolicy: &Policy{},
			expectedOutputPolicy: &Policy{
				ID:                 "",
				Min:                1,
				Max:                3,
				Enabled:            true,
				Cooldown:           10 * time.Millisecond,
				EvaluationInterval: 10 * time.Nanosecond,
				Checks: []*Check{
					{
						Name:   "approach-speed",
						Source: "front-sensor",
						Query:  "how-fast-am-i-going",
						Strategy: &Strategy{
							Name: "approach-velocity",
							Config: map[string]string{
								"target": "0.01ms",
							},
						},
					},
				},
				Target: &Target{
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
