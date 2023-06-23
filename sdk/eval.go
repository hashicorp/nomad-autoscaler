// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

// ScalingEvaluation forms an individual analysis undertaken by the autoscaler
// in order to determine the desired state of a target.
type ScalingEvaluation struct {
	ID               string
	Policy           *ScalingPolicy
	CheckEvaluations []*ScalingCheckEvaluation
	CreateTime       time.Time
}

// NewScalingEvaluation creates a new ScalingEvaluation based off the passed
// policy and status. It is responsible for hydrating all the fields to a basic
// level for safe usage throughout the scaling evaluation phase.
func NewScalingEvaluation(p *ScalingPolicy) *ScalingEvaluation {

	// Create the base eval.
	eval := ScalingEvaluation{
		ID:         uuid.Generate(),
		Policy:     p,
		CreateTime: time.Now().UTC(),
	}

	// Iterate the policy checks and add then to the eval.
	for _, check := range p.Checks {
		checkEval := ScalingCheckEvaluation{
			Check: check,
			Action: &ScalingAction{
				Meta: map[string]interface{}{
					"nomad_policy_id": p.ID,
				},
			},
		}

		// Ensure the Action is canonicalized so we don't need to perform this
		// again.
		checkEval.Action.Canonicalize()

		// Append the check.
		eval.CheckEvaluations = append(eval.CheckEvaluations, &checkEval)
	}

	return &eval
}
