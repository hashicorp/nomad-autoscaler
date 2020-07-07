package policy

import (
	"errors"
	"testing"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestPolicy_Validate(t *testing.T) {
	testCases := []struct {
		inputPolicy    *Policy
		expectedOutput error
		name           string
	}{
		{
			inputPolicy: &Policy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 1,
				Max: 10,
			},
			expectedOutput: nil,
			name:           "valid input policy",
		},
		{
			inputPolicy: &Policy{
				ID:  "",
				Min: 1,
				Max: 10,
			},
			expectedOutput: &multierror.Error{
				Errors: []error{
					errors.New("policy ID is empty"),
				},
			},
			name: "empty policy ID",
		},
		{
			inputPolicy: &Policy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: -1,
				Max: 10,
			},
			expectedOutput: &multierror.Error{
				Errors: []error{
					errors.New("policy Min can't be negative"),
				},
			},
			name: "negative minimum value",
		},
		{
			inputPolicy: &Policy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 100,
				Max: 10,
			},
			expectedOutput: &multierror.Error{
				Errors: []error{
					errors.New("policy Min must not be greater Max"),
				},
			},
			name: "policy minimum greater than maximum",
		},
		{
			inputPolicy: &Policy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 1,
				Max: -10,
			},
			expectedOutput: &multierror.Error{
				Errors: []error{
					errors.New("policy Max can't be negative"),
					errors.New("policy Min must not be greater Max"),
				},
			},
			name: "negative maximum value which is lower than minimum",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputPolicy.Validate()
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func TestCheck_CanonicalizeAPMQuery(t *testing.T) {
	testCases := []struct {
		inputCheck          *Check
		inputTarget         *Target
		expectedOutputCheck *Check
		name                string
	}{
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "prometheus",
				Query:  "scalar(super-data-point)",
			},
			inputTarget: nil,
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "prometheus",
				Query:  "scalar(super-data-point)",
			},
			name: "fully populated query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			inputTarget: nil,
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			name: "fully populated non-short node query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			inputTarget: nil,
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			name: "fully populated non-short taskgroup query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			inputTarget: &Target{
				Config: map[string]string{"Job": "example", "Group": "cache"},
			},
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			name: "correctly formatted taskgroup target short query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "percentage-allocated_memory",
			},
			inputTarget: &Target{
				Config: map[string]string{"node_class": "hashistack"},
			},
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			name: "correctly formatted node target short query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			inputTarget: &Target{
				Config: map[string]string{"Job": "example"},
			},
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			name: "incorrectly formatted taskgroup target short query",
		},
		{
			inputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "percentage-allocated_memory",
			},
			inputTarget: &Target{
				Config: map[string]string{},
			},
			expectedOutputCheck: &Check{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "percentage-allocated_memory",
			},
			name: "incorrectly formatted node target short query",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputCheck.CanonicalizeAPMQuery(tc.inputTarget)
			assert.Equal(t, tc.expectedOutputCheck, tc.inputCheck, tc.name)
		})
	}
}

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
