// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
)

func validateHorizontalPolicy(policy *api.ScalingPolicy) error {
	var result *multierror.Error

	// Validate Min and Max values.
	//   1. Min must not be nil.
	//   2. Min must be positive.
	//   3. Max must be positive.
	//   4. Max must not be nil.
	//   5. Min must be smaller than Max.
	if policy.Min == nil {
		result = multierror.Append(result, errors.New("scaling.min is missing"))
	} else if policy.Max == nil {
		result = multierror.Append(result, errors.New("scaling.max is missing"))
	} else {
		min := *policy.Min
		if min < 0 {
			result = multierror.Append(result, errors.New("scaling.min can't be negative"))
		}

		if min > *policy.Max {
			result = multierror.Append(result, errors.New("scaling.min must be smaller than scaling.max"))
		}

		if *policy.Max < 0 {
			result = multierror.Append(result, errors.New("scaling.max can't be negative"))
		}
	}

	// Validate Check blocks.
	err := validateBlocks(policy.Policy[keyChecks], "scaling.policy."+keyChecks, validateChecksHorizontal)
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
}

func validateChecksHorizontal(in map[string]interface{}, path string) error {
	return validateLabeledBlocks(in, path, ptr.IntToPtr(1), nil, validateCheckHorizontal)
}

func validateCheckHorizontal(c map[string]interface{}, path string, label string) error {
	// Nothing to check at this time
	return nil
}
