// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

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

func (nv *noopVerticalCheckRunner) runCheckAndCapCount(_ context.Context, currentCount int64, _ *queryMetricsCache) (action sdk.ScalingAction, err error) {
	action = sdk.ScalingAction{
		Direction: sdk.ScaleDirectionNone,
		Count:     currentCount,
	}

	action.CapCount(nv.policy.Min, nv.policy.Max)

	return action, nil
}

func (nv *noopVerticalCheckRunner) group() string {
	return ""
}

func (h *Handler) loadVerticalCheckRunner(policy *sdk.ScalingPolicy) (*noopVerticalCheckRunner, error) {
	return &noopVerticalCheckRunner{
		policy: policy,
	}, nil
}
