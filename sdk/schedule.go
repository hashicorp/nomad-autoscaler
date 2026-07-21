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
// Errors are field-relative (e.g. "start is required"); callers should wrap
// with their own path context (e.g. fmt.Errorf("policy.schedule: %w", err)).
func ValidateScalingPolicySchedule(s *ScalingPolicySchedule) error {
	if s == nil {
		return nil
	}

	if s.Start == "" {
		return fmt.Errorf("start is required")
	}

	hasEnd := s.End != ""
	hasDuration := s.Duration != ""

	if hasEnd == hasDuration {
		return fmt.Errorf("must define exactly one of end or duration")
	}

	if err := validateCron5Field(s.Start); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	if hasEnd {
		if err := validateCron5Field(s.End); err != nil {
			return fmt.Errorf("end: %w", err)
		}
	}

	if hasDuration {
		d, err := time.ParseDuration(s.Duration)
		if err != nil {
			return fmt.Errorf("duration must have time.Duration format, found %q", s.Duration)
		}
		if d <= 0 {
			return fmt.Errorf("duration must be greater than zero")
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
