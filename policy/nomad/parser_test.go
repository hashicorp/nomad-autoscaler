package nomad

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func Test_parsePolicy(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected sdk.ScalingPolicy
	}{
		{
			name:  "full scaling",
			input: "full-scaling",
			expected: sdk.ScalingPolicy{
				ID:                 "id",
				Min:                2,
				Max:                10,
				Enabled:            false,
				EvaluationInterval: 5 * time.Second,
				Cooldown:           5 * time.Minute,
				Type:               "horizontal",
				OnCheckError:       "fail",
				Target: &sdk.ScalingPolicyTarget{
					Name: "target",
					Config: map[string]string{
						"Namespace":   "default",
						"Job":         "full-scaling",
						"Group":       "test",
						"int_config":  "2",
						"bool_config": "true",
						"str_config":  "str",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:        "check-1",
						Source:      "source-1",
						Query:       "query-1",
						QueryWindow: time.Minute,
						OnError:     "ignore",
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy-1",
							Config: map[string]string{
								"int_config":  "2",
								"bool_config": "true",
								"str_config":  "str",
							},
						},
					},
					{
						Name:   "check-2",
						Group:  "group-2",
						Source: "source-2",
						Query:  "query-2",
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy-2",
							Config: map[string]string{
								"int_config":  "2",
								"bool_config": "true",
								"str_config":  "str",
							},
						},
					},
				},
			},
		},
		{
			name:  "minimum valid scaling",
			input: "minimum-valid-scaling",
			expected: sdk.ScalingPolicy{
				ID:      "id",
				Min:     1,
				Max:     10,
				Enabled: true,
				Type:    "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "minimum-valid-scaling",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:  "check",
						Query: "query",
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy",
							Config: map[string]string{
								"int_config":  "2",
								"bool_config": "true",
								"str_config":  "str",
							},
						},
					},
				},
			},
		},
		{
			name:     "missing scaling",
			input:    "missing-scaling",
			expected: sdk.ScalingPolicy{},
		},
		{
			name:  "empty policy",
			input: "empty-policy",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "empty-policy",
						"Group":     "test",
					},
				},
			},
		},
		{
			name:  "invalid evaluation_interval",
			input: "invalid-evaluation-interval",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "invalid-evaluation-interval",
						"Group":     "test",
					},
				},
			},
		},
		{
			name:  "invalid cooldown",
			input: "invalid-cooldown",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "invalid-cooldown",
						"Group":     "test",
					},
				},
			},
		},
		{
			name:  "empty target",
			input: "empty-target",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "target",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "empty-target",
						"Group":     "test",
					},
				},
			},
		},
		{
			name:  "invalid target",
			input: "invalid-target",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
			},
		},
		{
			name:  "empty check",
			input: "empty-check",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "empty-check",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{Name: "check"},
				},
			},
		},
		{
			name:  "single check",
			input: "single-check",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "single-check",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:   "check",
						Source: "source",
						Query:  "query",
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy",
							Config: map[string]string{
								"int_config":  "2",
								"bool_config": "true",
								"str_config":  "str",
							},
						},
					},
				},
			},
		},
		{
			name:  "invalid check",
			input: "invalid-check",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "invalid-check",
						"Group":     "test",
					},
				},
			},
		},
		{
			name:  "missing strategy",
			input: "missing-strategy",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "missing-strategy",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:   "check",
						Source: "source",
						Query:  "query",
					},
				},
			},
		},
		{
			name:  "empty strategy",
			input: "empty-strategy",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "empty-strategy",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name: "check",
						Strategy: &sdk.ScalingPolicyStrategy{
							Name:   "strategy",
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name:  "invalid strategy",
			input: "invalid-strategy",
			expected: sdk.ScalingPolicy{
				ID:   "id",
				Max:  10,
				Type: "horizontal",
				Target: &sdk.ScalingPolicyTarget{
					Name: "",
					Config: map[string]string{
						"Namespace": "default",
						"Job":       "invalid-strategy",
						"Group":     "test",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name: "check",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jobPath := fmt.Sprintf("test-fixtures/%s.json.golden", tc.input)
			job := TestParseJob(t, jobPath)

			if len(job.TaskGroups) != 1 {
				t.Fatalf("expected 1 group, found %d", len(job.TaskGroups))
			}

			actual := parsePolicy(job.TaskGroups[0].Scaling)

			// We assume check order is not relevant, so sort checks to avoid
			// flapping tests.
			if actual.Checks != nil {
				sort.Slice(actual.Checks, func(i, j int) bool {
					return actual.Checks[i].Name < actual.Checks[j].Name
				})
			}

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_parseBlock(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "valid block",
			input:    []interface{}{map[string]interface{}{}},
			expected: map[string]interface{}{},
		},
		{
			name:     "nil block",
			input:    nil,
			expected: nil,
		},
		{
			name:     "invalid root",
			input:    1,
			expected: nil,
		},
		{
			name:     "no element",
			input:    []interface{}{},
			expected: nil,
		},
		{
			name: "more than one element",
			input: []interface{}{
				map[string]interface{}{},
				map[string]interface{}{},
			},
			expected: nil,
		},
		{
			name:     "invalid element type",
			input:    []interface{}{1},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseBlock(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
