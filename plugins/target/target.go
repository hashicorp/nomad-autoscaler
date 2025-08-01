// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package target

import (
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// Target is the interface that all Target plugins are required to implement.
// The plugins are responsible for providing status details of the remote
// target, as well as carrying out scaling actions as decided by the Strategy
// plugin and internal autoscaler controls.
type Target interface {

	// Embed base.Base ensuring that strategy plugins implement this interface.
	base.Base

	TargetController
}

// Status collects and returns critical information of the status of the
// remote target. The information is used to understand whether the target
// is in a position to be scaled as well as the current running count which
// will be used when performing the strategy calculation.
// Scale triggers a scaling action against the remote target as specified
// by the config func argument.
type TargetController interface {
	Scale(action sdk.ScalingAction, config map[string]string) error
	Status(config map[string]string) (*sdk.TargetStatus, error)
}
