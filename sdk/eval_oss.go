// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package sdk

// ScalingCheckEvaluation is the evaluation of an individual policy check. Each
// check eval within a ScalingEvaluation is performed concurrently and a single
// "winner" picked once all have returned.
type ScalingCheckEvaluation struct {

	// Check is the individual ScalingPolicyCheck that this eval is concerned
	// with.
	Check *ScalingPolicyCheck

	// Metrics is the metric resulting from querying the APM.
	Metrics TimestampedMetrics

	// Action is the calculated desired state and is populated by strategy.Run.
	Action *ScalingAction
}
