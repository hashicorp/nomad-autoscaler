package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/helper/ptr"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_parsePolicy(t *testing.T) {
	testCases := []struct {
		name     string
		input    *api.ScalingPolicy
		expected policy.Policy
	}{
		{
			name: "full policy",
			input: &api.ScalingPolicy{
				ID:        "id",
				Namespace: "default",
				Target: map[string]string{
					"Namespace": "namespace",
					"Job":       "example",
					"Group":     "cache",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyEvaluationInterval: "5s",
					keyTarget: []interface{}{
						map[string]interface{}{
							"name": "target",
							"config": []interface{}{
								map[string]interface{}{
									"int_config": 2,
								},
							},
						},
					},
					keyChecks: []interface{}{
						map[string]interface{}{
							"check1": []interface{}{
								map[string]interface{}{
									keySource: "source1",
									keyQuery:  "query1",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"name": "strategy1",
											"config": []interface{}{
												map[string]interface{}{
													"bool_config": true,
												},
											},
										},
									},
								},
							},
							"check2": []interface{}{
								map[string]interface{}{
									keySource: "source2",
									keyQuery:  "query2",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"name": "strategy2",
											"config": []interface{}{
												map[string]interface{}{
													"int_config": 1,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Enabled: ptr.BoolToPtr(true),
			},
			expected: policy.Policy{
				ID:                 "id",
				Min:                1,
				Max:                5,
				Enabled:            true,
				EvaluationInterval: 5 * time.Second,
				Target: &policy.Target{
					Name: "target",
					Config: map[string]string{
						"Namespace":  "namespace",
						"Job":        "example",
						"Group":      "cache",
						"int_config": "2",
					},
				},
				Checks: []*policy.Check{
					{
						Name:   "check1",
						Source: "source1",
						Query:  "query1",
						Strategy: &policy.Strategy{
							Name: "strategy1",
							Config: map[string]string{
								"bool_config": "true",
							},
						},
					},
					{
						Name:   "check2",
						Source: "source2",
						Query:  "query2",
						Strategy: &policy.Strategy{
							Name: "strategy2",
							Config: map[string]string{
								"int_config": "1",
							},
						},
					},
				},
			},
		},
		{
			name: "policy with empty policy target",
			input: &api.ScalingPolicy{
				ID:        "id",
				Namespace: "default",
				Target: map[string]string{
					"Namespace": "namespace",
					"Job":       "example",
					"Group":     "cache",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:             "source",
					keyQuery:              "query",
					keyEvaluationInterval: "5s",
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"name": "strategy",
											"config": []interface{}{
												map[string]interface{}{
													"bool_config": true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Enabled: ptr.BoolToPtr(true),
			},
			expected: policy.Policy{
				ID:                 "id",
				Min:                1,
				Max:                5,
				Enabled:            true,
				EvaluationInterval: 5 * time.Second,
				Target: &policy.Target{
					Name: "",
					Config: map[string]string{
						"Namespace": "namespace",
						"Job":       "example",
						"Group":     "cache",
					},
				},
				Checks: []*policy.Check{
					{
						Name:   "check",
						Source: "source",
						Query:  "query",
						Strategy: &policy.Strategy{
							Name: "strategy",
							Config: map[string]string{
								"bool_config": "true",
							},
						},
					},
				},
			},
		},
		{
			name:  "empty policy",
			input: &api.ScalingPolicy{},
			expected: policy.Policy{
				Enabled: true,
			},
		},
		{
			name: "invalid strategy",
			input: &api.ScalingPolicy{
				Policy: map[string]interface{}{
					keyStrategy: true,
				},
			},
			expected: policy.Policy{
				Enabled: true,
			},
		},
		{
			name: "invalid target",
			input: &api.ScalingPolicy{
				Policy: map[string]interface{}{
					keyTarget: true,
				},
			},
			expected: policy.Policy{
				Enabled: true,
			},
		},
		{
			name: "policy with evaluation_interval",
			input: &api.ScalingPolicy{
				Policy: map[string]interface{}{
					keyEvaluationInterval: "7s",
				},
			},
			expected: policy.Policy{
				Enabled:            true,
				EvaluationInterval: 7 * time.Second,
			},
		},
		{
			name: "policy with cooldown",
			input: &api.ScalingPolicy{
				Policy: map[string]interface{}{
					keyCooldown: "7s",
				},
			},
			expected: policy.Policy{
				Enabled:  true,
				Cooldown: 7 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := parsePolicy(tc.input)
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
