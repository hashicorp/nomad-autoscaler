// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestStrategyPlugin_SetConfig(t *testing.T) {
	s := &StrategyPlugin{}
	expectedOutput := map[string]string{"example-item": "example-value"}
	err := s.SetConfig(expectedOutput)
	assert.Nil(t, err)
}

func TestStrategyPlugin_PluginInfo(t *testing.T) {
	s := &StrategyPlugin{}
	expectedOutput := &base.PluginInfo{Name: "pass-through", PluginType: "strategy"}
	actualOutput, err := s.PluginInfo()
	assert.Nil(t, err)
	assert.Equal(t, expectedOutput, actualOutput)
}

func TestStrategyPlugin_Run(t *testing.T) {
	testCases := []struct {
		inputEval     *sdk.ScalingCheckEvaluation
		inputCount    int64
		expectedResp  *sdk.ScalingCheckEvaluation
		expectedError error
		name          string
	}{
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_up": "not-the-int-you're-looking-for"},
					},
				},
			},
			expectedResp:  nil,
			expectedError: errors.New("invalid value for `max_scale_up`: not-the-int-you're-looking-for (string)"),
			name:          "incorrect input strategy config max_scale_up value",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_down": "not-the-int-you're-looking-for"},
					},
				},
			},
			expectedResp:  nil,
			expectedError: errors.New("invalid value for `max_scale_down`: not-the-int-you're-looking-for (string)"),
			name:          "incorrect input strategy config max_scale_down value",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     13,
					Direction: sdk.ScaleDirectionUp,
					Reason:    "scaling up because metric is 13",
				},
			},
			expectedError: nil,
			name:          "pass-through scale up",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 0}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 0}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     0,
					Direction: sdk.ScaleDirectionDown,
					Reason:    "scaling down because metric is 0",
				},
			},
			expectedError: nil,
			name:          "pass-through scale down to 0",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 10}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 10,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 10}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "no scaling - current count same as metric",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount:    10,
			expectedResp:  nil,
			expectedError: nil,
			name:          "no scaling - empty metric timeseries",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_up": "3"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_up": "3"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     5,
					Direction: sdk.ScaleDirectionUp,
					Reason:    "scaling up because metric is 13",
				},
			},
			expectedError: nil,
			name:          "pass-through scale up, but max_scale_up is set",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 3}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_down": "1"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 10,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 3}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"max_scale_down": "1"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     9,
					Direction: sdk.ScaleDirectionDown,
					Reason:    "scaling down because metric is 3",
				},
			},
			expectedError: nil,
			name:          "pass-through scale down, but max_scale_down is set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &StrategyPlugin{logger: hclog.NewNullLogger()}
			actualResp, actualError := s.Run(tc.inputEval, tc.inputCount)
			assert.Equal(t, tc.expectedResp, actualResp, tc.name)
			assert.Equal(t, tc.expectedError, actualError, tc.name)
		})
	}
}

func TestStrategyPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputCount     int64
		inputMetric    float64
		threshold      float64
		expectedOutput sdk.ScaleDirection
	}{
		{inputCount: 0, inputMetric: 1, expectedOutput: sdk.ScaleDirectionUp},
		{inputCount: 5, inputMetric: 5, expectedOutput: sdk.ScaleDirectionNone},
		{inputCount: 4, inputMetric: 0, expectedOutput: sdk.ScaleDirectionDown},
	}

	s := &StrategyPlugin{}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, s.calculateDirection(tc.inputCount, tc.inputMetric))
	}
}
