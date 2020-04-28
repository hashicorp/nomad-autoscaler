package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_validateScalingPolicy(t *testing.T) {
	testCases := []struct {
		name        string
		input       *api.ScalingPolicy
		expectError bool
	}{
		{
			name:        "valid policy",
			input:       validPolicy(),
			expectError: false,
		},
		{
			name: "valid policy without optional values",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Policy[keySource] = ""
				p.Enabled = nil
				return p
			}(),
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
			name: "invalid target",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Target = nil
				return p
			}(),
			expectError: true,
		},
		{
			name: "invalid policy",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Policy = nil
				return p
			}(),
			expectError: true,
		},
		{
			name: "min is nil",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Min = nil
				return p
			}(),
			expectError: true,
		},
		{
			name: "min is negative",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Min = int64ToPtr(-1)
				return p
			}(),
			expectError: true,
		},
		{
			name: "max is negative",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Max = -1
				return p
			}(),
			expectError: true,
		},
		{
			name: "max less than min",
			input: func() *api.ScalingPolicy {
				p := validPolicy()
				p.Max = 1
				p.Min = int64ToPtr(2)
				return p
			}(),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateScalingPolicy(tc.input)

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}

func Test_validateTarget(t *testing.T) {
	testCases := []struct {
		name        string
		input       map[string]string
		expectError bool
	}{
		{
			name: "target is valid",
			input: map[string]string{
				"Job":   "example",
				"Group": "cache",
			},
			expectError: false,
		},
		{
			name:        "target is nil",
			input:       nil,
			expectError: true,
		},
		{
			name: "target.job is missing",
			input: map[string]string{
				"Group": "cache",
			},
			expectError: true,
		},
		{
			name: "target.group is missing",
			input: map[string]string{
				"Job": "example",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTarget(tc.input)

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}

func Test_validatePolicy(t *testing.T) {
	testCases := []struct {
		name        string
		input       map[string]interface{}
		expectError bool
	}{
		{
			name: "policy is valid",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: false,
		},
		{
			name: "policy without optional values is valid",
			input: map[string]interface{}{
				keyQuery: "query",
				keyStrategy: []interface{}{map[string]interface{}{
					"name": "strategy",
				}},
			},
			expectError: false,
		},
		{
			name:        "policy is nil",
			input:       nil,
			expectError: true,
		},
		{
			name: "policy.source is not a string",
			input: map[string]interface{}{
				keySource:             2,
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.query is missing",
			input: map[string]interface{}{
				keySource:             "source",
				keyEvaluationInterval: "5s",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.query is not a string",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              2,
				keyEvaluationInterval: "5s",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.query is empty",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "",
				keyEvaluationInterval: "5s",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.evaluation_interval has wrong type",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: 5,
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.evaluation_interval has wrong format",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5 seconds",
				keyStrategy: []interface{}{map[string]interface{}{
					"name":   "strategy",
					"config": map[string]string{"key": "value"},
				}},
			},
			expectError: true,
		},
		{
			name: "policy.strategy is missing",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
			},
			expectError: true,
		},
		{
			name: "policy.strategy has wrong type",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy:           true,
			},
			expectError: true,
		},
		{
			name: "policy.strategy is empty",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy:           []interface{}{},
			},
			expectError: true,
		},
		{
			name: "policy.strategy has more than 1 element",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy:           []interface{}{1, 2},
			},
			expectError: true,
		},
		{
			name: "policy.strategy[0] has wrong type",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy:           []interface{}{1},
			},
			expectError: true,
		},
		{
			name: "policy.strategy[0] is invalid",
			input: map[string]interface{}{
				keySource:             "source",
				keyQuery:              "query",
				keyEvaluationInterval: "5s",
				keyStrategy:           []interface{}{map[string]interface{}{}},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePolicy(tc.input)

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}

func Test_validateStrategy(t *testing.T) {
	testCases := []struct {
		name        string
		input       map[string]interface{}
		expectError bool
	}{
		{
			name: "strategy is valid",
			input: map[string]interface{}{
				"name":   "strategy",
				"config": map[string]string{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "strategy without optional values is valid",
			input: map[string]interface{}{
				"name": "strategy",
			},
			expectError: false,
		},
		{
			name:        "strategy is nil",
			input:       nil,
			expectError: true,
		},
		{
			name: "strategy.name is missing",
			input: map[string]interface{}{
				"config": map[string]string{"key": "value"},
			},
			expectError: true,
		},
		{
			name: "strategy.name is not a string",
			input: map[string]interface{}{
				"name":   2,
				"config": map[string]string{"key": "value"},
			},
			expectError: true,
		},
		{
			name: "strategy.name is empty",
			input: map[string]interface{}{
				"name":   "",
				"config": map[string]string{"key": "value"},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStrategy(tc.input)

			assertFunc := assert.NoError
			if tc.expectError {
				assertFunc = assert.Error
			}

			assertFunc(t, err)
		})
	}
}

func int64ToPtr(i int64) *int64 {
	return &i
}

func boolToPtr(b bool) *bool {
	return &b
}

func validPolicy() *api.ScalingPolicy {
	return &api.ScalingPolicy{
		ID:        "id",
		Namespace: "default",
		Target: map[string]string{
			"Job":   "example",
			"Group": "cache",
		},
		Min: int64ToPtr(1),
		Max: 5,
		Policy: map[string]interface{}{
			keySource: "source",
			keyQuery:  "query",
			keyStrategy: []interface{}{map[string]interface{}{
				"name":   "strategy",
				"config": map[string]string{"key": "value"},
			}},
		},
		Enabled: boolToPtr(true),
	}
}
