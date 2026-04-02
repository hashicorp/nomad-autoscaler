// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

func TestCompiledSchedule_ActiveAt_EndSchedule(t *testing.T) {
	s, err := compileSchedule(&sdk.ScalingPolicySchedule{
		Start: "0 14 * * *",
		End:   "0 15 * * *",
	})
	must.NoError(t, err)

	tests := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{
			name:   "before_start",
			now:    time.Date(2026, 1, 1, 13, 59, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "start_boundary_inclusive",
			now:    time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC),
			active: true,
		},
		{
			name:   "inside_window",
			now:    time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC),
			active: true,
		},
		{
			name:   "just_before_end",
			now:    time.Date(2026, 1, 1, 14, 59, 59, 0, time.UTC),
			active: true,
		},
		{
			name:   "end_boundary_exclusive",
			now:    time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "inactive_after_window_before_next_start",
			now:    time.Date(2026, 1, 1, 16, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "next_day_start_active_again",
			now:    time.Date(2026, 1, 2, 14, 0, 0, 0, time.UTC),
			active: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.active, s.activeAt(tc.now))
		})
	}
}

func TestCompiledSchedule_ActiveAt_DurationSchedule(t *testing.T) {
	s, err := compileSchedule(&sdk.ScalingPolicySchedule{
		Start:    "0 14 * * *",
		Duration: "1h",
	})
	must.NoError(t, err)

	tests := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{
			name:   "before_start",
			now:    time.Date(2026, 1, 1, 13, 59, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "start_boundary_inclusive",
			now:    time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC),
			active: true,
		},
		{
			name:   "inside_window",
			now:    time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC),
			active: true,
		},
		{
			name:   "just_before_duration_end",
			now:    time.Date(2026, 1, 1, 14, 59, 59, 0, time.UTC),
			active: true,
		},
		{
			name:   "duration_end_boundary_exclusive",
			now:    time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "inactive_after_window_before_next_start",
			now:    time.Date(2026, 1, 1, 16, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name:   "next_day_start_active_again",
			now:    time.Date(2026, 1, 2, 14, 0, 0, 0, time.UTC),
			active: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.active, s.activeAt(tc.now))
		})
	}
}

func TestCompiledSchedule_ActiveAt_EndSchedule_EdgeCases(t *testing.T) {
	overnightSchedule := &sdk.ScalingPolicySchedule{
		Start: "30 16 * * *",
		End:   "0 9 * * *",
	}

	weekdayWindowSchedule := &sdk.ScalingPolicySchedule{
		Start: "0 0 * * Mon",
		End:   "59 23 * * Fri",
	}

	leapDaySchedule := &sdk.ScalingPolicySchedule{
		Start: "0 0 29 2 *",
		End:   "0 12 29 2 *",
	}

	monthEndSchedule := &sdk.ScalingPolicySchedule{
		Start: "0 0 31 * *",
		End:   "0 12 31 * *",
	}

	tests := []struct {
		name     string
		schedule *sdk.ScalingPolicySchedule
		now      time.Time
		active   bool
	}{
		{
			name:     "overnight_after_start",
			schedule: overnightSchedule,
			now:      time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC),
			active:   true,
		},
		{
			name:     "overnight_before_end",
			schedule: overnightSchedule,
			now:      time.Date(2026, 1, 2, 8, 59, 0, 0, time.UTC),
			active:   true,
		},
		{
			name:     "overnight_end_boundary_exclusive",
			schedule: overnightSchedule,
			now:      time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC),
			active:   false,
		},
		{
			name:     "overnight_before_start",
			schedule: overnightSchedule,
			now:      time.Date(2026, 1, 1, 16, 29, 0, 0, time.UTC),
			active:   false,
		},
		{
			name:     "weekday_window_midweek_active",
			schedule: weekdayWindowSchedule,
			now:      time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC), // Wednesday
			active:   true,
		},
		{
			name:     "weekday_window_weekend_inactive",
			schedule: weekdayWindowSchedule,
			now:      time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC), // Saturday
			active:   false,
		},
		{
			name:     "weekday_window_start_boundary_inclusive",
			schedule: weekdayWindowSchedule,
			now:      time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), // Monday start
			active:   true,
		},
		{
			name:     "weekday_window_end_boundary_exclusive",
			schedule: weekdayWindowSchedule,
			now:      time.Date(2026, 1, 9, 23, 59, 0, 0, time.UTC), // Friday end minute
			active:   false,
		},
		{
			name: "mismatched_frequency",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 1 * * Mon",
				End:   "0 2 * * *",
			},
			now:    time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name:     "leap_day_window_active_before_end",
			schedule: leapDaySchedule,
			now:      time.Date(2028, 2, 29, 11, 59, 0, 0, time.UTC),
			active:   true,
		},
		{
			name:     "leap_day_window_end_boundary_exclusive",
			schedule: leapDaySchedule,
			now:      time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC),
			active:   false,
		},
		{
			name:     "month_end_window_active_before_end",
			schedule: monthEndSchedule,
			now:      time.Date(2026, 1, 31, 11, 59, 0, 0, time.UTC),
			active:   true,
		},
		{
			name:     "month_end_window_inactive_on_non_matching_day",
			schedule: monthEndSchedule,
			now:      time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
			active:   false,
		},
		{
			name: "no_prior_end_yet_after_first_start",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 0 1 1 *",
				End:   "0 0 2 1 *",
			},
			now:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			active: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := compileSchedule(tc.schedule)
			must.NoError(t, err)
			must.Eq(t, tc.active, s.activeAt(tc.now))
		})
	}
}
