// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package policy

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type HistoricalAPMGetter interface{}

type noopHistoricalAPMGetter struct{}

type noopVerticalCheckRunner struct {
	policy *sdk.ScalingPolicy
}

func (nv *noopVerticalCheckRunner) runCheckAndCapCount(_ context.Context, currentCount int64) (sdk.ScalingAction, error) {
	a := sdk.ScalingAction{
		Direction: sdk.ScaleDirectionNone,
		Count:     currentCount,
	}

	a.CapCount(nv.policy.Min, nv.policy.Max)

	return a, nil
}

func (nv *noopVerticalCheckRunner) group() string {
	return ""
}

func (h *Handler) loadVerticalCheckRunner() (*noopVerticalCheckRunner, error) {
	return &noopVerticalCheckRunner{
		policy: h.policy,
	}, nil
}
