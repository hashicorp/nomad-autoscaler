package policyeval

import "github.com/hashicorp/nomad-autoscaler/sdk"

type Pipeline interface {
	Status(*sdk.ScalingEvaluation) error
	Query(*sdk.ScalingEvaluation) error
	Strategy(*sdk.ScalingEvaluation) error
	Scale(*sdk.ScalingEvaluation) error
}
