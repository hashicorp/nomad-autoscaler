package file

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func Test_decodeFile(t *testing.T) {
	testCases := []struct {
		inputFile            string
		inputPolicy          *sdk.ScalingPolicy
		expectedOutputPolicy *sdk.ScalingPolicy
		expectedOutputError  error
		name                 string
	}{
		{
			inputFile:   "./test-fixtures/full-cluster-policy.hcl",
			inputPolicy: &sdk.ScalingPolicy{},
			expectedOutputPolicy: &sdk.ScalingPolicy{
				ID:                 "",
				Enabled:            true,
				Min:                10,
				Max:                100,
				Cooldown:           10 * time.Minute,
				EvaluationInterval: 1 * time.Minute,
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:        "cpu_nomad",
						Source:      "nomad_apm",
						Query:       "cpu_high-memory",
						QueryWindow: time.Minute,
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "target-value",
							Config: map[string]string{
								"target": "80",
							},
						},
					},
					{
						Name:   "memory_prom",
						Source: "prometheus",
						Query:  "nomad_client_allocated_memory*100/(nomad_client_allocated_memory+nomad_client_unallocated_memory)",
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
			expectedOutputError: nil,
			name:                "full parsable cluster scaling policy",
		},
		{
			inputFile:   "./test-fixtures/full-task-group-policy.hcl",
			inputPolicy: &sdk.ScalingPolicy{},
			expectedOutputPolicy: &sdk.ScalingPolicy{
				ID:                 "",
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
			expectedOutputError: nil,
			name:                "full parsable task group scaling policy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := decodeFile(tc.inputFile, tc.inputPolicy)
			assert.Equal(t, tc.expectedOutputPolicy, tc.inputPolicy, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)

			// Print unexpected errors.
			if actualError != nil && tc.expectedOutputError == nil {
				t.Logf("%s", actualError)
			}
		})
	}
}
