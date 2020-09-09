package sdk

// ScalingEvaluation forms an individual analysis undertaken by the autoscaler
// in order to determine the desired state of a target.
type ScalingEvaluation struct {
	Policy           *ScalingPolicy
	TargetStatus     *TargetStatus
	CheckEvaluations []*ScalingCheckEvaluation
}

// ScalingCheckEvaluation is the evaluation of an individual policy check. Each
// check eval within a ScalingEvaluation is performed concurrently and a single
// "winner" picked once all have returned.
type ScalingCheckEvaluation struct {

	// Check is the individual ScalingPolicyCheck that this eval is concerned
	// with.
	Check *ScalingPolicyCheck

	// Metric is the metric resulting from querying the APM.
	Metric float64

	// Action is the calculated desired state and is populated by strategy.Run.
	Action *ScalingAction
}

// NewScalingEvaluation creates a new ScalingEvaluation based off the passed
// policy and status. It is responsible for hydrating all the fields to a basic
// level for safe usage throughout the scaling evaluation phase.
func NewScalingEvaluation(p *ScalingPolicy, status *TargetStatus) *ScalingEvaluation {

	// Create the base eval.
	eval := ScalingEvaluation{Policy: p, TargetStatus: status}

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
