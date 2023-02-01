// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	strategy := &Noop{logger: hclog.Default()}

	cases := []struct {
		name     string
		config   map[string]string
		expected *sdk.ScalingAction
	}{
		{
			name: "scale up",
			config: map[string]string{
				runConfigKeyCount:     "10",
				runConfigKeyReason:    "why not?",
				runConfigKeyError:     "false",
				runConfigKeyDirection: "up",
			},
			expected: &sdk.ScalingAction{
				Count:     10,
				Reason:    "why not?",
				Error:     false,
				Direction: sdk.ScaleDirectionUp,
			},
		},
		{
			name: "scale down",
			config: map[string]string{
				runConfigKeyCount:     "10",
				runConfigKeyReason:    "why not?",
				runConfigKeyError:     "false",
				runConfigKeyDirection: "down",
			},
			expected: &sdk.ScalingAction{
				Count:     10,
				Reason:    "why not?",
				Error:     false,
				Direction: sdk.ScaleDirectionDown,
			},
		},
		{
			name: "scale error",
			config: map[string]string{
				runConfigKeyReason: "didn't work",
				runConfigKeyError:  "true",
			},
			expected: &sdk.ScalingAction{
				Reason: "didn't work",
				Error:  true,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert := assert.New(t)

			eval := &sdk.ScalingCheckEvaluation{
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: c.config,
					},
				},
			}

			got, err := strategy.Run(eval, 0)
			assert.NoError(err)
			assert.Equal(c.expected, got.Action)
		})
	}
}
