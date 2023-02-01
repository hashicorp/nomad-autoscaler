// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"errors"
	"testing"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestProcessor_ValidatePolicy(t *testing.T) {
	testCases := []struct {
		inputPolicy    *sdk.ScalingPolicy
		expectedOutput error
		name           string
	}{
		{
			inputPolicy: &sdk.ScalingPolicy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 1,
				Max: 10,
			},
			expectedOutput: nil,
			name:           "valid input policy",
		},
		{
			inputPolicy: &sdk.ScalingPolicy{
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
			inputPolicy: &sdk.ScalingPolicy{
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
			inputPolicy: &sdk.ScalingPolicy{
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
			inputPolicy: &sdk.ScalingPolicy{
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
		{
			inputPolicy: &sdk.ScalingPolicy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 1,
				Max: 10,
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{"datacenter": "eu-east-17"},
				},
			},
			expectedOutput: nil,
			name:           "valid datacenter horizontal cluster scaling policy",
		},
		{
			inputPolicy: &sdk.ScalingPolicy{
				ID:  "ce888afe-3dd2-144c-7227-74644434f708",
				Min: 1,
				Max: 10,
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{"node_class": "puppy"},
				},
			},
			expectedOutput: nil,
			name:           "valid node_class horizontal cluster scaling policy",
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
		inputCheck          *sdk.ScalingPolicyCheck
		inputAPMNames       []string
		inputTarget         *sdk.ScalingPolicyTarget
		expectedOutputCheck *sdk.ScalingPolicyCheck
		name                string
	}{
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "prometheus",
				Query:  "scalar(super-data-point)",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "prometheus",
				Query:  "scalar(super-data-point)",
			},
			name: "fully populated query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			name: "fully populated non-short node query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget:   nil,
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			name: "fully populated non-short taskgroup query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget: &sdk.ScalingPolicyTarget{
				Config: map[string]string{"Job": "example", "Group": "cache"},
			},
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "taskgroup_avg_cpu/cache/example",
			},
			name: "correctly formatted taskgroup target short query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "percentage-allocated_memory",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget: &sdk.ScalingPolicyTarget{
				Config: map[string]string{"node_class": "hashistack"},
			},
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "node_percentage-allocated_memory/hashistack/class",
			},
			name: "correctly formatted node target short query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget: &sdk.ScalingPolicyTarget{
				Config: map[string]string{"Job": "example"},
			},
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "avg_cpu",
			},
			name: "incorrectly formatted taskgroup target short query",
		},
		{
			inputCheck: &sdk.ScalingPolicyCheck{
				Name:   "random-check",
				Source: "nomad-apm",
				Query:  "percentage-allocated_memory",
			},
			inputAPMNames: []string{"nomad-apm"},
			inputTarget: &sdk.ScalingPolicyTarget{
				Config: map[string]string{},
			},
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
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
		inputPolicy          *sdk.ScalingPolicy
		inputDefaults        *ConfigDefaults
		expectedOutputPolicy *sdk.ScalingPolicy
		name                 string
	}{
		{
			inputPolicy: &sdk.ScalingPolicy{
				Cooldown: 20 * time.Second,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &sdk.ScalingPolicy{
				Cooldown:           20 * time.Second,
				EvaluationInterval: 5 * time.Second,
			},
			name: "evaluation interval set to default",
		},
		{
			inputPolicy: &sdk.ScalingPolicy{
				EvaluationInterval: 15 * time.Second,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           11 * time.Second,
			},
			expectedOutputPolicy: &sdk.ScalingPolicy{
				Cooldown:           11 * time.Second,
				EvaluationInterval: 15 * time.Second,
			},
			name: "cooldown set to default",
		},
		{
			inputPolicy: &sdk.ScalingPolicy{},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &sdk.ScalingPolicy{
				Cooldown:           10 * time.Second,
				EvaluationInterval: 5 * time.Second,
			},
			name: "evaluation interval and cooldown set to default",
		},
		{
			inputPolicy: &sdk.ScalingPolicy{
				Cooldown:           10 * time.Minute,
				EvaluationInterval: 5 * time.Minute,
			},
			inputDefaults: &ConfigDefaults{
				DefaultEvaluationInterval: 5 * time.Second,
				DefaultCooldown:           10 * time.Second,
			},
			expectedOutputPolicy: &sdk.ScalingPolicy{
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
