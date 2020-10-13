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
	TargetStatus     *TargetStatus
	CheckEvaluations []*ScalingCheckEvaluation
	CreateTime       time.Time
}

// NewScalingEvaluation creates a new ScalingEvaluation based off the passed
// policy and status. It is responsible for hydrating all the fields to a basic
// level for safe usage throughout the scaling evaluation phase.
func NewScalingEvaluation(p *ScalingPolicy, status *TargetStatus) *ScalingEvaluation {

	// Create the base eval.
	eval := ScalingEvaluation{
		ID:           uuid.Generate(),
		Policy:       p,
		TargetStatus: status,
		CreateTime:   time.Now().UTC(),
	}

	// Iterate the policy checks and add then to the eval.
	for _, check := range p.Checks {
		checkEval := ScalingCheckEvaluation{
			Check:  check,
			Action: &ScalingAction{},
		}

		// Ensure the Action is canonicalized so we don't need to perform this
		// again.
		checkEval.Action.Canonicalize()

		// Append the check.
		eval.CheckEvaluations = append(eval.CheckEvaluations, &checkEval)
	}

	return &eval
}
