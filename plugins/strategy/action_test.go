package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAction_SetDryRun(t *testing.T) {
	testCases := []struct {
		inputAction          *Action
		expectedOutputAction *Action
		name                 string
	}{
		{
			inputAction: &Action{
				Count: int64ToPointer(3),
				Meta:  map[string]interface{}{},
			},
			expectedOutputAction: &Action{
				Count: nil,
				Meta: map[string]interface{}{
					"nomad_autoscaler.dry_run":       true,
					"nomad_autoscaler.dry_run.count": int64(3),
				},
			},
			name: "count greater than zero",
		},
		{
			inputAction: &Action{
				Meta: map[string]interface{}{},
			},
			expectedOutputAction: &Action{
				Count: nil,
				Meta:  map[string]interface{}{"nomad_autoscaler.dry_run": true}},
			name: "count not set",
		},
	}

	for _, tc := range testCases {
		tc.inputAction.SetDryRun()
		assert.Equal(t, tc.expectedOutputAction, tc.inputAction, tc.name)
	}
}

func int64ToPointer(i int64) *int64 { return &i }
