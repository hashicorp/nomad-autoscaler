package policy

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_Canonicalize(t *testing.T) {
	assert := assert.New(t)

	from := &api.ScalingPolicy{
		Enabled: nil,
		Policy: map[string]interface{}{
			KeyEvaluationInterval: "2s",
		},
	}

	to := &Policy{
		Query:    "avg_cpu",
		Target:   &Target{},
		Strategy: &Strategy{},
	}

	Canonicalize(from, to)

	// from.Enable: nil -> to.Enabled: true
	assert.True(to.Enabled)

	// from.EvaluationInterval: string -> to.EvaluationInterval: time.Duration
	assert.Equal(2*time.Second, to.EvaluationInterval)

	// from.Source: "" -> to.Source: "nomad-apm"
	assert.Equal(plugins.InternalAPMNomad, to.Source)

	// from.Target.Name: "" -> to.Target.Name: "nomad-target"
	assert.Equal(plugins.InternalTargetNomad, to.Target.Name)
}

func Test_ValidateEvaluationInterval(t *testing.T) {
	testCases := []struct {
		input         interface{}
		expectedError bool
	}{
		{input: nil, expectedError: false},
		{input: "2s", expectedError: false},
		{input: "", expectedError: true},
		{input: "not valid", expectedError: true},
	}

	for _, tc := range testCases {
		p := &api.ScalingPolicy{
			Policy: map[string]interface{}{KeyEvaluationInterval: tc.input},
		}

		hasError := false
		errs := Validate(p)
		for _, e := range errs {
			if strings.Contains(e.Error(), KeyEvaluationInterval) {
				hasError = true
				break
			}
		}
		assert.Equal(t, tc.expectedError, hasError, "input: %#v", tc.input)
	}
}

func Test_parseStrategy(t *testing.T) {
	testCases := []struct {
		inputStrategy  interface{}
		expectedOutput *Strategy
	}{
		{
			inputStrategy: []interface{}{
				map[string]interface{}{
					"name": "target-value",
					"config": []interface{}{
						map[string]interface{}{"target": float64(20)},
					},
				},
			},
			expectedOutput: &Strategy{
				Name:   "target-value",
				Config: map[string]string{"target": "20"},
			},
		},
	}

	for _, tc := range testCases {
		actualOutput := ParseStrategy(tc.inputStrategy)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

func Test_parseTarget(t *testing.T) {
	testCases := []struct {
		inputTarget    interface{}
		expectedOutput *Target
		name           string
	}{
		{
			inputTarget:    nil,
			expectedOutput: &Target{},
			name:           "nil passed target interface",
		},
	}

	for _, tc := range testCases {
		actualOutput := ParseTarget(tc.inputTarget)
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
	}
}
