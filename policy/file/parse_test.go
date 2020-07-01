package file

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/stretchr/testify/assert"
)

func Test_decodeFile(t *testing.T) {
	testCases := []struct {
		inputFile            string
		inputPolicy          *policy.Policy
		expectedOutputPolicy *policy.Policy
		expectedOutputError  error
		name                 string
	}{
		{
			inputFile:   "./test-fixtures/full-cluster-policy.hcl",
			inputPolicy: &policy.Policy{},
			expectedOutputPolicy: &policy.Policy{
				ID:                 "",
				Enabled:            true,
				Min:                10,
				Max:                100,
				Cooldown:           10 * time.Minute,
				EvaluationInterval: 1 * time.Minute,
				Checks: []*policy.Check{
					{
						Name:   "cpu_nomad",
						Source: "nomad_apm",
						Query:  "cpu_high-memory",
						Strategy: &policy.Strategy{
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
						Strategy: &policy.Strategy{
							Name: "target-value",
							Config: map[string]string{
								"target": "80",
							},
						},
					},
				},
				Target: &policy.Target{
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := decodeFile(tc.inputFile, tc.inputPolicy)
			assert.Equal(t, tc.expectedOutputPolicy, tc.inputPolicy, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
		})
	}
}
