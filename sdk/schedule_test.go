// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_validateSchedule(t *testing.T) {
	tests := []struct {
		name      string
		schedule  *ScalingPolicySchedule
		errorText string
	}{
		{
			name:      "nil schedule",
			errorText: "",
		},
		{
			name: "valid start end",
			schedule: &ScalingPolicySchedule{
				Start: "0 9 * * *",
				End:   "0 17 * * *",
			},
			errorText: "",
		},
		{
			name: "valid start duration",
			schedule: &ScalingPolicySchedule{
				Start:    "0 9 * * *",
				Duration: "8h",
			},
			errorText: "",
		},
		{
			name: "missing start",
			schedule: &ScalingPolicySchedule{
				End: "0 17 * * *",
			},
			errorText: "start is required",
		},
		{
			name: "both end and duration",
			schedule: &ScalingPolicySchedule{
				Start:    "0 9 * * *",
				End:      "0 17 * * *",
				Duration: "8h",
			},
			errorText: "exactly one of end or duration",
		},
		{
			name: "invalid cron field count",
			schedule: &ScalingPolicySchedule{
				Start:    "0 9 * *",
				Duration: "8h",
			},
			errorText: "strict 5-field cron",
		},
		{
			name: "non-positive duration",
			schedule: &ScalingPolicySchedule{
				Start:    "0 9 * * *",
				Duration: "0s",
			},
			errorText: "greater than zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSchedule(tc.schedule, "policy.schedule")
			if tc.errorText == "" {
				must.NoError(t, err)
				return
			}
			must.ErrorContains(t, err, tc.errorText)
		})
	}
}
