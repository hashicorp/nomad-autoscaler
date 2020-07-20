package policy

import (
	"errors"
	"testing"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestProcessor_ValidatePolicy(t *testing.T) {
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

	pr := Processor{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := pr.ValidatePolicy(tc.inputPolicy)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func TestProcessor_CanonicalizeAPMQuery(t *testing.T) {
	testCases := []struct {
		inputCheck          *Check
		inputAPMNames       []string
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
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
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
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
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
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
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
			inputAPMNames: []string{"nomad-apm"},
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
			inputAPMNames: []string{"nomad-apm"},
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
			inputAPMNames: []string{"nomad-apm"},
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
			inputAPMNames: []string{"nomad-apm"},
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
			pr := Processor{nomadAPMs: tc.inputAPMNames}
			pr.CanonicalizeAPMQuery(tc.inputCheck, tc.inputTarget)
			assert.Equal(t, tc.expectedOutputCheck, tc.inputCheck, tc.name)
		})
	}
}

func TestProcessor_ApplyPolicyDefaults(t *testing.T) {
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
			pr := Processor{defaults: tc.inputDefaults}
			pr.ApplyPolicyDefaults(tc.inputPolicy)
			assert.Equal(t, tc.expectedOutputPolicy, tc.inputPolicy, tc.name)
		})
	}
}

func TestProcessor_isNomadAPMQuery(t *testing.T) {
	testCases := []struct {
		inputProcessor *Processor
		inputSource    string
		expectedOutput bool
		name           string
	}{
		{
			inputProcessor: &Processor{
				nomadAPMs: nil,
			},
			inputSource:    "nagios",
			expectedOutput: false,
			name:           "no nomad APMs configured",
		},
		{
			inputProcessor: &Processor{
				nomadAPMs: []string{"nomad-apm"},
			},
			inputSource:    "nagios",
			expectedOutput: false,
			name:           "source doesn't match Nomad APM name",
		},
		{
			inputProcessor: &Processor{
				nomadAPMs: []string{"nomad-apm"},
			},
			inputSource:    "nomad-apm",
			expectedOutput: true,
			name:           "source does match default Nomad APM name",
		},
		{
			inputProcessor: &Processor{
				nomadAPMs: []string{"nomad-platform-apm"},
			},
			inputSource:    "nomad-platform-apm",
			expectedOutput: true,
			name:           "source does match non-default Nomad APM name",
		},
		{
			inputProcessor: &Processor{
				nomadAPMs: []string{"nomad-qa-apm", "nomad-platform-apm", "nomad-support-apm"},
			},
			inputSource:    "nomad-platform-apm",
			expectedOutput: true,
			name:           "source does match non-default Nomad APM name in list of APM name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputProcessor.isNomadAPMQuery(tc.inputSource)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
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
