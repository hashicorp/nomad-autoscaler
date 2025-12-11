// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nomad

import "github.com/hashicorp/nomad/api"

func validateClusterPolicy(policy *api.ScalingPolicy) error {
	// Use the same validation rules for now.
	return validateHorizontalPolicy(policy)
}
