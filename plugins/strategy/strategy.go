// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package strategy

import (
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// Strategy is the interface that all Strategy plugins are required to
// implement. The Strategy plugin is responsible to performing calculations
// that produce a desired state based on a number of input parameters.
type Strategy interface {

	// Embed base.Base ensuring that strategy plugins implement this interface.
	base.Base

	// Run triggers a run of the strategy calculation. It is responsible for
	// populating the sdk.ScalingAction object within the passed eval and
	// returning the eval to the caller. The count input variable represents
	// the current state of the scaling target.
	Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error)
}
