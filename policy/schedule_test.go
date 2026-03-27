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

	must.False(t, s.activeAt(time.Date(2026, 1, 1, 13, 59, 0, 0, time.UTC)))
	must.True(t, s.activeAt(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)))
	must.True(t, s.activeAt(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC)))
	must.False(t, s.activeAt(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC)))
}

func TestCompiledSchedule_ActiveAt_DurationSchedule(t *testing.T) {
	s, err := compileSchedule(&sdk.ScalingPolicySchedule{
		Start:    "0 14 * * *",
		Duration: "1h",
	})
	must.NoError(t, err)

	must.False(t, s.activeAt(time.Date(2026, 1, 1, 13, 59, 0, 0, time.UTC)))
	must.True(t, s.activeAt(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)))
	must.True(t, s.activeAt(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC)))
	must.False(t, s.activeAt(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC)))
}
