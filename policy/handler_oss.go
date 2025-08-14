// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package policy

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type HistoricalAPMGetter interface{}

type noopHistoricalAPMGetter struct{}

func (h *Handler) configureVerticalPolicy() error {
	return nil
}

func (h *Handler) updateVerticalPolicy(up *sdk.ScalingPolicy) error {
	return h.configureVerticalPolicy()
}
