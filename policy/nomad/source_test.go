package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestSourceConfig_canonicalize(t *testing.T) {
	testCases := []struct {
		name     string
		input    *SourceConfig
		expected *SourceConfig
	}{
		{
			name:  "empty config",
			input: &SourceConfig{},
			expected: &SourceConfig{
				DefaultEvaluationInterval: policy.DefaultEvaluationInterval,
			},
		},
		{
			name: "don't overwrite values",
			input: &SourceConfig{
				DefaultEvaluationInterval: time.Second,
			},
			expected: &SourceConfig{
				DefaultEvaluationInterval: time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.canonicalize()
			assert.Equal(t, tc.expected, tc.input)
		})
	}
}

func TestSource_canonicalizePolicy(t *testing.T) {
	testCases := []struct {
		name     string
		input    *policy.Policy
		expected *policy.Policy
		cb       func(*api.Config, *SourceConfig)
	}{
		{
			name: "full policy",
			input: &policy.Policy{
				ID:                 "string",
				Min:                1,
				Max:                5,
				Source:             "source",
				Query:              "query",
				Enabled:            true,
				EvaluationInterval: time.Second,
				Target: &policy.Target{
					Name: "target",
					Config: map[string]string{
						"target_config":  "yes",
						"target_config2": "no",
						"Job":            "job",
						"Group":          "group",
					},
				},
				Strategy: &policy.Strategy{
					Name: "strategy",
					Config: map[string]string{
						"strategy_config1": "yes",
						"strategy_config2": "no",
					},
				},
			},
			expected: &policy.Policy{
				ID:                 "string",
				Min:                1,
				Max:                5,
				Source:             "source",
				Query:              "query",
				Enabled:            true,
				EvaluationInterval: time.Second,
				Target: &policy.Target{
					Name: "target",
					Config: map[string]string{
						"target_config":  "yes",
						"target_config2": "no",
						"Job":            "job",
						"Group":          "group",
						"job_id":         "job",
						"group":          "group",
					},
				},
				Strategy: &policy.Strategy{
					Name: "strategy",
					Config: map[string]string{
						"strategy_config1": "yes",
						"strategy_config2": "no",
					},
				},
			},
		},
		{
			name:  "set all defaults",
			input: &policy.Policy{},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name:   plugins.InternalTargetNomad,
					Config: map[string]string{},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "expand query when source is empty",
			input: &policy.Policy{
				Query: "avg_cpu",
				Target: &policy.Target{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
			},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				Query:              "avg_cpu/group/job",
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "job",
						"group":  "group",
					},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name: "expand query when source is nomad apm",
			input: &policy.Policy{
				Source: plugins.InternalAPMNomad,
				Query:  "avg_cpu",
				Target: &policy.Target{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
			},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				Query:              "avg_cpu/group/job",
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "job",
						"group":  "group",
					},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name: "expand query from user-defined values",
			input: &policy.Policy{
				Query: "avg_cpu",
				Target: &policy.Target{
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "my_job",
						"group":  "my_group",
					},
				},
			},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				Query:              "avg_cpu/my_group/my_job",
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "my_job",
						"group":  "my_group",
					},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name: "don't expand query if not nomad apm",
			input: &policy.Policy{
				Source: "not_nomad",
				Query:  "avg_cpu",
				Target: &policy.Target{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
			},
			expected: &policy.Policy{
				Source:             "not_nomad",
				Query:              "avg_cpu",
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "job",
						"group":  "group",
					},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name: "don't expand query if not in short format",
			input: &policy.Policy{
				Query: "avg_cpu/my_group/my_job",
				Target: &policy.Target{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
			},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				Query:              "avg_cpu/my_group/my_job",
				EvaluationInterval: 10 * time.Second,
				Target: &policy.Target{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":    "job",
						"Group":  "group",
						"job_id": "job",
						"group":  "group",
					},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
		},
		{
			name:  "sets eval interval from agent",
			input: &policy.Policy{},
			expected: &policy.Policy{
				Source:             plugins.InternalAPMNomad,
				EvaluationInterval: 5 * time.Second,
				Target: &policy.Target{
					Name:   plugins.InternalTargetNomad,
					Config: map[string]string{},
				},
				Strategy: &policy.Strategy{
					Config: map[string]string{},
				},
			},
			cb: func(_ *api.Config, sourceConfig *SourceConfig) {
				sourceConfig.DefaultEvaluationInterval = 5 * time.Second
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := TestNomadSource(t, tc.cb)
			s.canonicalizePolicy(tc.input)
			assert.Equal(t, tc.expected, tc.input)
		})
	}
}
