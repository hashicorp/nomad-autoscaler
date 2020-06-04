package agent

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/stretchr/testify/assert"
)

func TestAgent_enforceActionLimits(t *testing.T) {
	testCases := []struct {
		inputPolicy          *policy.Policy
		inputAction          strategy.Action
		inputStatus          *target.Status
		expectedOutputAction strategy.Action
		expectedOutputError  error
		name                 string
	}{
		{
			inputPolicy: &policy.Policy{
				Min: 2,
				Max: 20,
				Target: &policy.Target{
					Config: map[string]string{"max_change": "4"},
				},
			},
			inputAction: strategy.Action{
				Count:  21,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			inputStatus: &target.Status{Count: 15},
			expectedOutputAction: strategy.Action{
				Count:  19,
				Reason: "modified desired count from 20 to 19 to limit change to 4",
				Error:  false,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.capped":   true,
					"nomad_autoscaler.count.original": int64(21),
					"nomad_autoscaler.reason_history": []string{
						"capped count from 21 to 20 to stay within limits",
					},
				},
			},
			expectedOutputError: nil,
			name:                "desired count capped and change limited",
		},
		{
			inputPolicy: &policy.Policy{
				Min: 2,
				Max: 20,
				Target: &policy.Target{
					Config: map[string]string{"max_change": "4"},
				},
			},
			inputAction: strategy.Action{
				Count:  21,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			inputStatus: &target.Status{Count: 19},
			expectedOutputAction: strategy.Action{
				Count:  20,
				Reason: "capped count from 21 to 20 to stay within limits",
				Error:  false,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.capped":   true,
					"nomad_autoscaler.count.original": int64(21),
					"nomad_autoscaler.reason_history": []string{},
				},
			},
			expectedOutputError: nil,
			name:                "desired count capped only",
		},
		{
			inputPolicy: &policy.Policy{
				Min: 2,
				Max: 20,
				Target: &policy.Target{
					Config: map[string]string{"max_change": "5"},
				},
			},
			inputAction: strategy.Action{
				Count:  20,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			inputStatus: &target.Status{Count: 10},
			expectedOutputAction: strategy.Action{
				Count:  15,
				Reason: "modified desired count from 20 to 15 to limit change to 5",
				Error:  false,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.original": int64(20),
					"nomad_autoscaler.reason_history": []string{},
				},
			},
			expectedOutputError: nil,
			name:                "desired count limited only",
		},
		{
			inputPolicy: &policy.Policy{
				Min: 2,
				Max: 20,
				Target: &policy.Target{
					Config: map[string]string{"max_change": "3"},
				},
			},
			inputAction: strategy.Action{
				Count:  20,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			inputStatus: &target.Status{Count: 18},
			expectedOutputAction: strategy.Action{
				Count:  20,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			expectedOutputError: nil,
			name:                "no enforcement required",
		},
		{
			inputPolicy: &policy.Policy{
				Min: 2,
				Max: 20,
				Target: &policy.Target{
					Config: map[string]string{"max_change": "three"},
				},
			},
			inputAction: strategy.Action{
				Count:  20,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			inputStatus: &target.Status{Count: 18},
			expectedOutputAction: strategy.Action{
				Count:  20,
				Reason: "",
				Error:  false,
				Meta:   map[string]interface{}{},
			},
			expectedOutputError: errors.New("failed to convert max_change value to int64: strconv.ParseInt: parsing \"three\": invalid syntax"),
			name:                "incorrectly formatted max_change parameter",
		},
	}

	a := Agent{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := a.enforceActionLimits(tc.inputPolicy, &tc.inputAction, tc.inputStatus)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
			assert.Equal(t, tc.expectedOutputAction, tc.inputAction, tc.name)
		})
	}
}
