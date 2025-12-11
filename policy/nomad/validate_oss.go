// Copyright IBM Corp. 2020, 2025
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
