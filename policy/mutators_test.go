// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestPolicyMutators_NomadAPMMUtator(t *testing.T) {
	testCases := []struct {
		name     string
		input    *sdk.ScalingPolicy
		expected Mutations
	}{
		{
			name: "no mutation for app scaling",
			input: &sdk.ScalingPolicy{
				Min:  0,
				Type: sdk.ScalingPolicyTypeHorizontal,
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source: "nomad-apm",
					},
				},
			},
			expected: Mutations{},
		},
		{
			name: "no mutation for non-zero cluster scaling",
			input: &sdk.ScalingPolicy{
				Min:  1,
				Type: sdk.ScalingPolicyTypeCluster,
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source: "nomad-apm",
					},
				},
			},
			expected: Mutations{},
		},
		{
			name: "cluster scaling to zero",
			input: &sdk.ScalingPolicy{
				Min:  0,
				Type: sdk.ScalingPolicyTypeCluster,
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source: "nomad-apm",
					},
				},
			},
			expected: Mutations{"min value set to 1 since scaling cluster to 0 is not supported by the Nomad APM"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := NomadAPMMutator{}
			got := m.MutatePolicy(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
