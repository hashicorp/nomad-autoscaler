// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package nomad

import "github.com/hashicorp/nomad-autoscaler/sdk"

func (s *Source) canonicalizeAdditionalTypes(p *sdk.ScalingPolicy) {}
