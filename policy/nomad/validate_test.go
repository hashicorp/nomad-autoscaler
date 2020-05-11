package nomad

import (
	"flag"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/helper/ptr"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

var showValidationError = flag.Bool("show-validation-error", false, "")

func Test_validateScalingPolicy(t *testing.T) {
	testCases := []struct {
		name        string
		input       *api.ScalingPolicy
		expectError bool
	}{
		{
			name: "valid policy",
			input: &api.ScalingPolicy{
				ID:        "id",
				Namespace: "default",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:             "source",
					keyQuery:              "query",
					keyEvaluationInterval: "5s",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
							"config": []interface{}{
								map[string]interface{}{
									"key": "value",
								},
							},
						},
					},
					keyTarget: []interface{}{
						map[string]interface{}{
							"name": "target",
							"config": []interface{}{
								map[string]interface{}{
									"key": "value",
								},
							},
						},
					},
				},
				Enabled: ptr.BoolToPtr(true),
			},
			expectError: false,
		},
		{
			name: "valid policy without optional values",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil policy",
			input:       nil,
			expectError: true,
		},
		{
			name:        "empty policy",
			input:       &api.ScalingPolicy{},
			expectError: true,
		},
		{
			name: "id is missing",
			input: &api.ScalingPolicy{
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "target is missing",
			input: &api.ScalingPolicy{
				ID:  "id",
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "min is missing",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "min is negative",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(-1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "max is negative",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: -5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "max less than min",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(2),
				Max: 1,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy is missing",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
			},
			expectError: true,
		},
		{
			name: "policy.source is not a string",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource: 2,
					keyQuery:  "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.query is missing",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource: "source",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.query is not a string",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource: "source",
					keyQuery:  5,
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.query is empty",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource: "source",
					keyQuery:  "",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.evaluation_interval has wrong type",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:             "source",
					keyQuery:              "query",
					keyEvaluationInterval: 5,
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.evaluation_interval has wrong format",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:             "source",
					keyQuery:              "query",
					keyEvaluationInterval: "5 seconds",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.strategy is missing",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource: "source",
					keyQuery:  "query",
				},
			},
			expectError: true,
		},
		{
			name: "policy.strategy.name is missing",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.strategy.name is not a string",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": 2,
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.strategy.name is empty",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.strategy.config has wrong type",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name":   "strategy",
							"config": "not a block",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.target.name is not a string",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
					keyTarget: []interface{}{
						map[string]interface{}{
							"name": 2,
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.target.name is empty",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
					keyTarget: []interface{}{
						map[string]interface{}{
							"name": "",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.target.config has wrong type",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keyQuery: "query",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},

					keyTarget: []interface{}{
						map[string]interface{}{
							"name":   "strategy",
							"config": "not a block",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.cooldown has wrong type",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:   "source",
					keyQuery:    "query",
					keyCooldown: 5,
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.cooldown has wrong format",
			input: &api.ScalingPolicy{
				ID: "id",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Int64ToPtr(1),
				Max: 5,
				Policy: map[string]interface{}{
					keySource:   "source",
					keyQuery:    "query",
					keyCooldown: "5 seconds",
					keyStrategy: []interface{}{
						map[string]interface{}{
							"name": "strategy",
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateScalingPolicy(tc.input)
			if err != nil && *showValidationError {
				fmt.Println(err)
			}

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}

func Test_validateBlock(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		validator   func(in map[string]interface{}) error
		expectError bool
	}{
		{
			name: "valid block",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name: "valid block map",
			input: map[string]interface{}{
				"key": "value",
			},
			expectError: false,
		},
		{
			name:        "block root has wrong type",
			input:       true,
			expectError: true,
		},
		{
			name:        "block root is empty",
			input:       []interface{}{},
			expectError: true,
		},
		{
			name: "block root has more than 1 element",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
				1,
			},
			expectError: true,
		},
		{
			name:        "block root first element has wront type",
			input:       []interface{}{1},
			expectError: true,
		},
		{
			name: "validator is called",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
			},
			validator: func(in map[string]interface{}) error {
				return fmt.Errorf("error from validator")
			},
			expectError: true,
		},
		{
			name: "validator is called with map",
			input: map[string]interface{}{
				"key": "value",
			},
			validator: func(in map[string]interface{}) error {
				return fmt.Errorf("error from validator")
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBlock(tc.input, "path", "key", tc.validator)
			if err != nil && *showValidationError {
				fmt.Println(err)
			}

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}
