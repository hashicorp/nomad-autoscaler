package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScaleDirection_String(t *testing.T) {
	testCases := []struct {
		inputDirection       ScaleDirection
		expectedOutputString string
	}{
		{inputDirection: ScaleDirectionNone, expectedOutputString: "none"},
		{inputDirection: ScaleDirectionDown, expectedOutputString: "down"},
		{inputDirection: ScaleDirectionUp, expectedOutputString: "up"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedOutputString, func(t *testing.T) {
			actualOutput := tc.inputDirection.String()
			assert.Equal(t, tc.expectedOutputString, actualOutput, tc.expectedOutputString)
		})
	}
}

func TestAction_Canonicalize(t *testing.T) {
	testCases := []struct {
		inputAction          *Action
		expectedOutputAction *Action
		name                 string
	}{
		{
			inputAction:          &Action{},
			expectedOutputAction: &Action{Meta: map[string]interface{}{}},
			name:                 "empty input action",
		},
		{
			inputAction:          &Action{Meta: map[string]interface{}{"foo": "bar"}},
			expectedOutputAction: &Action{Meta: map[string]interface{}{"foo": "bar"}},
			name:                 "populated input action meta",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputAction.Canonicalize()
			assert.Equal(t, tc.expectedOutputAction, tc.inputAction)
		})
	}
}

func TestAction_SetDryRun(t *testing.T) {
	testCases := []struct {
		inputAction          *Action
		expectedOutputAction *Action
		name                 string
	}{
		{
			inputAction: &Action{
				Count: 3,
				Meta:  map[string]interface{}{},
			},
			expectedOutputAction: &Action{
				Count: -1,
				Meta: map[string]interface{}{
					"nomad_autoscaler.dry_run":       true,
					"nomad_autoscaler.dry_run.count": int64(3),
				},
			},
			name: "count greater than zero",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputAction.SetDryRun()
			assert.Equal(t, tc.expectedOutputAction, tc.inputAction)
		})
	}
}

func TestAction_CapCount(t *testing.T) {
	testCases := []struct {
		inputAction          *Action
		inputMin             int64
		inputMax             int64
		expectedOutputAction *Action
		name                 string
	}{
		{
			inputAction:          &Action{},
			inputMin:             0,
			inputMax:             0,
			expectedOutputAction: &Action{},
			name:                 "empty input action",
		},
		{
			inputAction: &Action{
				Count: 4,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &Action{
				Count: 5,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.capped":   true,
					"nomad_autoscaler.count.original": int64(4),
					"nomad_autoscaler.reason_history": []string{},
				},
				Reason: "capped count from 4 to 5 to stay within limits",
			},
			name: "desired count lower than min threshold",
		},
		{
			inputAction: &Action{
				Count: 15,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &Action{
				Count: 10,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.capped":   true,
					"nomad_autoscaler.count.original": int64(15),
					"nomad_autoscaler.reason_history": []string{},
				},
				Reason: "capped count from 15 to 10 to stay within limits",
			},
			name: "desired count higher than max threshold",
		},
		{
			inputAction: &Action{
				Count:  0,
				Meta:   map[string]interface{}{},
				Reason: "scaled to 0",
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &Action{
				Count: 5,
				Meta: map[string]interface{}{
					"nomad_autoscaler.count.capped":   true,
					"nomad_autoscaler.count.original": int64(0),
					"nomad_autoscaler.reason_history": []string{"scaled to 0"},
				},
				Reason: "capped count from 0 to 5 to stay within limits",
			},
			name: "store previous reason",
		},
		{
			inputAction: &Action{
				Count: 7,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &Action{
				Count: 7,
				Meta:  map[string]interface{}{},
			},
			name: "desired count doesn't break thresholds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputAction.CapCount(tc.inputMin, tc.inputMax)
			assert.Equal(t, tc.expectedOutputAction, tc.inputAction)
		})
	}
}

func TestAction_pushReason(t *testing.T) {
	testCases := []struct {
		inputAction          *Action
		inputReason          string
		expectedOutputAction *Action
		name                 string
	}{
		{
			inputAction: &Action{Meta: map[string]interface{}{}},
			inputReason: "capped count from 0 to 1 to stay within limits",
			expectedOutputAction: &Action{
				Reason: "capped count from 0 to 1 to stay within limits",
				Meta: map[string]interface{}{
					"nomad_autoscaler.reason_history": []string{},
				},
			},
			name: "no existing reason history",
		},

		{
			inputAction: &Action{
				Reason: "capped count from 0 to 1 to stay within limits",
				Meta:   map[string]interface{}{},
			},
			inputReason: "capped count from 10 to 20 to stay within limits",
			expectedOutputAction: &Action{
				Meta: map[string]interface{}{
					"nomad_autoscaler.reason_history": []string{
						"capped count from 0 to 1 to stay within limits",
					},
				},
				Reason: "capped count from 10 to 20 to stay within limits",
			},
			name: "existing reason history",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputAction.pushReason(tc.inputReason)
			assert.Equal(t, tc.expectedOutputAction, tc.inputAction)
		})
	}
}
