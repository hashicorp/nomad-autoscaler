// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import "github.com/hashicorp/nomad/api"

func validateClusterPolicy(policy *api.ScalingPolicy) error {
	// Use the same validation rules for now.
	return validateHorizontalPolicy(policy)
}
