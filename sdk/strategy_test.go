// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package sdk

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
		inputAction          *ScalingAction
		expectedOutputAction *ScalingAction
		name                 string
	}{
		{
			inputAction:          &ScalingAction{},
			expectedOutputAction: &ScalingAction{Meta: map[string]interface{}{}},
			name:                 "empty input action",
		},
		{
			inputAction:          &ScalingAction{Meta: map[string]interface{}{"foo": "bar"}},
			expectedOutputAction: &ScalingAction{Meta: map[string]interface{}{"foo": "bar"}},
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
		inputAction          *ScalingAction
		expectedOutputAction *ScalingAction
		name                 string
	}{
		{
			inputAction: &ScalingAction{
				Count: 3,
				Meta:  map[string]interface{}{},
			},
			expectedOutputAction: &ScalingAction{
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
		inputAction          *ScalingAction
		inputMin             int64
		inputMax             int64
		expectedOutputAction *ScalingAction
		name                 string
	}{
		{
			inputAction:          &ScalingAction{},
			inputMin:             0,
			inputMax:             0,
			expectedOutputAction: &ScalingAction{},
			name:                 "empty input action",
		},
		{
			inputAction: &ScalingAction{
				Count: 4,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &ScalingAction{
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
			inputAction: &ScalingAction{
				Count: 15,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &ScalingAction{
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
			inputAction: &ScalingAction{
				Count:  0,
				Meta:   map[string]interface{}{},
				Reason: "scaled to 0",
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &ScalingAction{
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
			inputAction: &ScalingAction{
				Count: 7,
				Meta:  map[string]interface{}{},
			},
			inputMin: 5,
			inputMax: 10,
			expectedOutputAction: &ScalingAction{
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
		inputAction          *ScalingAction
		inputReason          string
		expectedOutputAction *ScalingAction
		name                 string
	}{
		{
			inputAction: &ScalingAction{Meta: map[string]interface{}{}},
			inputReason: "capped count from 0 to 1 to stay within limits",
			expectedOutputAction: &ScalingAction{
				Reason: "capped count from 0 to 1 to stay within limits",
				Meta: map[string]interface{}{
					"nomad_autoscaler.reason_history": []string{},
				},
			},
			name: "no existing reason history",
		},

		{
			inputAction: &ScalingAction{
				Reason: "capped count from 0 to 1 to stay within limits",
				Meta:   map[string]interface{}{},
			},
			inputReason: "capped count from 10 to 20 to stay within limits",
			expectedOutputAction: &ScalingAction{
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

func TestPreemptAction(t *testing.T) {
	testCases := []struct {
		name     string
		a        *ScalingAction
		b        *ScalingAction
		expected *ScalingAction
	}{
		{
			name: "none vs none",
			a: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
		},
		{
			name: "none vs down",
			a: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionDown,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
		},
		{
			name: "none vs up",
			a: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "down vs none",
			a: &ScalingAction{
				Direction: ScaleDirectionDown,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
		},
		{
			name: "down vs down",
			a: &ScalingAction{
				Count:     1,
				Direction: ScaleDirectionDown,
			},
			b: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
		},
		{
			name: "down vs down reverse",
			a: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
			b: &ScalingAction{
				Count:     1,
				Direction: ScaleDirectionDown,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
		},
		{
			name: "down vs down same count",
			a: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
			b: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionDown,
			},
		},
		{
			name: "down vs up",
			a: &ScalingAction{
				Direction: ScaleDirectionDown,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "up vs none",
			a: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionNone,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "up vs down",
			a: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
			b: &ScalingAction{
				Direction: ScaleDirectionDown,
			},
			expected: &ScalingAction{
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "up vs up",
			a: &ScalingAction{
				Count:     1,
				Direction: ScaleDirectionUp,
			},
			b: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "up vs up reverse",
			a: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
			b: &ScalingAction{
				Count:     1,
				Direction: ScaleDirectionUp,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
		},
		{
			name: "up vs up same count",
			a: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
			b: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
			expected: &ScalingAction{
				Count:     2,
				Direction: ScaleDirectionUp,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := PreemptScalingAction(tc.a, tc.b)
			assert.Equal(t, actual, tc.expected)
		})
	}
}
