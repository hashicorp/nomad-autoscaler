// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/cronexpr"
)

// ValidateScalingPolicySchedule validates a policy/check schedule definition.
// path is a caller-provided field prefix used to contextualize returned errors
// (for example: policy.schedule or check <name>.schedule).
func ValidateScalingPolicySchedule(s *ScalingPolicySchedule, path string) error {
	if s == nil {
		return nil
	}

	if s.Start == "" {
		return fmt.Errorf("%s.start is required", path)
	}

	hasEnd := s.End != ""
	hasDuration := s.Duration != ""

	if hasEnd == hasDuration {
		return fmt.Errorf("%s must define exactly one of end or duration", path)
	}

	if err := validateCron5Field(s.Start); err != nil {
		return fmt.Errorf("%s.start: %w", path, err)
	}

	if hasEnd {
		if err := validateCron5Field(s.End); err != nil {
			return fmt.Errorf("%s.end: %w", path, err)
		}
	}

	if hasDuration {
		d, err := time.ParseDuration(s.Duration)
		if err != nil {
			return fmt.Errorf("%s.duration must have time.Duration format, found %q", path, s.Duration)
		}
		if d <= 0 {
			return fmt.Errorf("%s.duration must be greater than zero", path)
		}
	}

	return nil
}

func validateCron5Field(expr string) error {
	if len(strings.Fields(expr)) != 5 {
		return fmt.Errorf("must use strict 5-field cron format")
	}
	if _, err := cronexpr.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	return nil
}
