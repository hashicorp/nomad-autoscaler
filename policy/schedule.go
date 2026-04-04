// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/cronexpr"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// compiledSchedule is the cached runtime representation of a schedule.
type compiledSchedule struct {
	startExpr *cronexpr.Expression
	endExpr   *cronexpr.Expression
	duration  time.Duration
	usesEnd   bool // true if this schedule uses an end time, false if it uses a duration
}

// lastOccurrenceSearchHorizonYears bounds the backward search used to approximate the
// last cron occurrence because cronexpr exposes Next() but not Last().
const lastOccurrenceSearchHorizonYears = 32

func compileSchedule(s *sdk.ScalingPolicySchedule) (*compiledSchedule, error) {
	if s == nil {
		return nil, nil
	}

	if err := validateScheduleDefinition(s); err != nil {
		return nil, err
	}

	startExpr, err := cronexpr.Parse(s.Start)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schedule.start: %w", err)
	}

	compiled := &compiledSchedule{startExpr: startExpr}

	if s.End != "" {
		endExpr, err := cronexpr.Parse(s.End)
		if err != nil {
			return nil, fmt.Errorf("failed to parse schedule.end: %w", err)
		}
		compiled.usesEnd = true
		compiled.endExpr = endExpr
		return compiled, nil
	}

	d, err := time.ParseDuration(s.Duration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schedule.duration: %w", err)
	}
	compiled.duration = d

	return compiled, nil
}

func (s *compiledSchedule) activeAt(now time.Time) bool {
	if s == nil {
		return true
	}

	nowUTC := now.UTC()

	if s.usesEnd {
		lastStart, ok := lastOccurrenceAtOrBefore(s.startExpr, nowUTC)
		if !ok {
			return false
		}

		lastEnd, ok := lastOccurrenceAtOrBefore(s.endExpr, nowUTC)
		if !ok {
			return true
		}

		return lastStart.After(lastEnd)
	}

	windowStart := nowUTC.Add(-s.duration).Add(time.Nanosecond)
	nextStart := s.startExpr.Next(windowStart)
	return !nextStart.After(nowUTC)
}

func lastOccurrenceAtOrBefore(expr *cronexpr.Expression, now time.Time) (time.Time, bool) {
	now = now.UTC() // just to be sure
	searchStart := now.AddDate(-lastOccurrenceSearchHorizonYears, 0, 0)
	first := expr.Next(searchStart.Add(-time.Nanosecond))
	if first.IsZero() || first.After(now) {
		return time.Time{}, false
	}

	low := searchStart.UnixNano() - 1
	high := now.UnixNano()
	last := first

	// binary search
	for low <= high {
		mid := low + (high-low)/2
		next := expr.Next(time.Unix(0, mid).UTC())
		if !next.After(now) {
			last = next
			low = mid + 1
			continue
		}

		high = mid - 1
	}

	return last, true
}

func validateScheduleDefinition(s *sdk.ScalingPolicySchedule) error {
	if s.Start == "" {
		return fmt.Errorf("schedule.start is required")
	}

	hasEnd := s.End != ""
	hasDuration := s.Duration != ""
	if hasEnd == hasDuration {
		return fmt.Errorf("schedule must define exactly one of end or duration")
	}

	if len(strings.Fields(s.Start)) != 5 {
		return fmt.Errorf("schedule.start must use strict 5-field cron format")
	}

	if hasEnd && len(strings.Fields(s.End)) != 5 {
		return fmt.Errorf("schedule.end must use strict 5-field cron format")
	}

	if hasDuration {
		d, err := time.ParseDuration(s.Duration)
		if err != nil {
			return fmt.Errorf("schedule.duration must have time.Duration format")
		}
		if d <= 0 {
			return fmt.Errorf("schedule.duration must be greater than zero")
		}
	}

	return nil
}
