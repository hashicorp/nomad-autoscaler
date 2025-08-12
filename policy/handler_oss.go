// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package policy

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func (h *Handler) configureVerticalPolicy() error {

	h.calculateNewCount = func(ctx context.Context, currentCount int64) (sdk.ScalingAction, error) {
		return sdk.ScalingAction{
			Count:  currentCount,
			Reason: "Vertical scaling is not supported in OSS mode",
		}, nil
	}
	return nil
}

func (h *Handler) updateVerticalPolicy(up *sdk.ScalingPolicy) error {
	return h.configureVerticalPolicy()
}
