// +build !ent

package sdk

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
