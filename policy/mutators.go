// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// Mutations is a list of human-friendly descriptions of the changes performed
// by a mutator.
type Mutations []string

// Mutator is an interface used to apply changes to a scaling policy.
type Mutator interface {
	MutatePolicy(*sdk.ScalingPolicy) Mutations
}

// NomadAPMMutator handles the special case for zero count cluster scaling
// since it can't query nodes if there are none running.
type NomadAPMMutator struct{}

func (m NomadAPMMutator) MutatePolicy(p *sdk.ScalingPolicy) Mutations {
	result := Mutations{}

	if p.Type != sdk.ScalingPolicyTypeCluster || p.Min != 0 {
		return result
	}

	for _, c := range p.Checks {
		if c.Source == "nomad-apm" {
			p.Min = 1
			result = append(result, "min value set to 1 since scaling cluster to 0 is not supported by the Nomad APM")
			break
		}
	}

	return result
}
