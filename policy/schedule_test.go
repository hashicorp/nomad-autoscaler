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
	tests := []struct {
		name     string
		schedule *sdk.ScalingPolicySchedule
		now      time.Time
		active   bool
	}{
		{
			name: "overnight_after_start",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "30 16 * * *",
				End:   "0 9 * * *",
			},
			now:    time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC),
			active: true,
		},
		{
			name: "overnight_before_end",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "30 16 * * *",
				End:   "0 9 * * *",
			},
			now:    time.Date(2026, 1, 2, 8, 59, 0, 0, time.UTC),
			active: true,
		},
		{
			name: "overnight_end_boundary_exclusive",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "30 16 * * *",
				End:   "0 9 * * *",
			},
			now:    time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC),
			active: false,
		},
		{
			name: "overnight_before_start",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "30 16 * * *",
				End:   "0 9 * * *",
			},
			now:    time.Date(2026, 1, 1, 16, 29, 0, 0, time.UTC),
			active: false,
		},
		{
			name: "weekday_window_midweek_active",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 0 * * 1",
				End:   "59 23 * * 5",
			},
			now:    time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC), // Wednesday
			active: true,
		},
		{
			name: "weekday_window_weekend_inactive",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 0 * * 1",
				End:   "59 23 * * 5",
			},
			now:    time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC), // Saturday
			active: false,
		},
		{
			name: "weekday_window_start_boundary_inclusive",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 0 * * 1",
				End:   "59 23 * * 5",
			},
			now:    time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), // Monday start
			active: true,
		},
		{
			name: "weekday_window_end_boundary_exclusive",
			schedule: &sdk.ScalingPolicySchedule{
				Start: "0 0 * * 1",
				End:   "59 23 * * 5",
			},
			now:    time.Date(2026, 1, 9, 23, 59, 0, 0, time.UTC), // Friday end minute
			active: false,
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
