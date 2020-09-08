package sdk

// ScalingEvaluation forms an individual analysis undertaken by the autoscaler
// in order to determine the desired state of a target.
type ScalingEvaluation struct {
	Policy       *ScalingPolicy
	TargetStatus *TargetStatus
}
