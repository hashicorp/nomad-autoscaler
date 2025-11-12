// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

func additionalPolicyTypeValidation(policy *api.ScalingPolicy) error {
	return fmt.Errorf("policy type %q not supported", policy.Type)
}
