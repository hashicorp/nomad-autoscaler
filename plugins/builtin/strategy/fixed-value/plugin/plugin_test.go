package plugin

import (
	"fmt"
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
	assert.Equal(t, expectedOutput, s.config)
}

func TestStrategyPlugin_PluginInfo(t *testing.T) {
	s := &StrategyPlugin{}
	expectedOutput := &base.PluginInfo{Name: "fixed-value", PluginType: "strategy"}
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
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{},
				},
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("missing required field `value`"),
			name:          "incorrect strategy input config",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "not-the-int-you're-looking-for"},
					},
				},
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("invalid value for `value`: not-the-int-you're-looking-for (string)"),
			name:          "incorrect input strategy config fixed value - string",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "13"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "13"},
					},
				},
				Action: &sdk.ScalingAction{
					Count: 13,
					Reason: "scaling up because fixed value is 13",
					Direction: sdk.ScaleDirectionUp,
				},
			},
			expectedError: nil,
			name:          "fixed scale up",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 26}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "4"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 10,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 26}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "4"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     4,
					Reason: "scaling down because fixed value is 4",
					Direction: sdk.ScaleDirectionDown,
				},
			},
			expectedError: nil,
			name:          "fixed scale down",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 9}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "10"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 10,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 9}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"value": "10"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "no scaling - current count same as fixed",
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
		fixedCount     int64
		expectedOutput sdk.ScaleDirection
	}{
		{inputCount: 0, fixedCount: 1, expectedOutput: sdk.ScaleDirectionUp},
		{inputCount: 5, fixedCount: 5, expectedOutput: sdk.ScaleDirectionNone},
		{inputCount: 4, fixedCount: 0, expectedOutput: sdk.ScaleDirectionDown},
	}

	s := &StrategyPlugin{}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, s.calculateDirection(tc.inputCount, tc.fixedCount))
	}
}
