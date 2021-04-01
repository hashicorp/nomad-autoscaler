package plugin

import (
	"fmt"
	"testing"
	"time"

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
	expectedOutput := &base.PluginInfo{Name: "target-value", PluginType: "strategy"}
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
					Strategy: &sdk.ScalingPolicyStrategy{},
				},
				Action: &sdk.ScalingAction{},
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("missing required field `target`"),
			name:          "incorrect strategy input config",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "not-the-float-you're-looking-for"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("invalid value for `target`: not-the-float-you're-looking-for (string)"),
			name:          "incorrect input strategy config target value",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "0", "threshold": "not-the-float-you're-looking-for"},
					},
				},
			},
			expectedResp:  nil,
			expectedError: fmt.Errorf("invalid value for `threshold`: not-the-float-you're-looking-for (string)"),
			name:          "incorrect input strategy config threshold value",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "empty metrics",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 13}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "factor equals 1 with non-zero count",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 26}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 26}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "13"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     4,
					Reason:    "scaling up because factor is 2.000000",
					Direction: sdk.ScaleDirectionUp,
				},
			},
			expectedError: nil,
			name:          "factor greater than 1 with non-zero count",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 20}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 0,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 20}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     2,
					Reason:    "scaling up because factor is 2.000000",
					Direction: sdk.ScaleDirectionUp,
				},
			},
			expectedError: nil,
			name:          "scale from 0 with non-zero target",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 9}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 1,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 9}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "no scaling based on small target/metric difference",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 1}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 0,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 1}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "10"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     1,
					Reason:    "scaling up because factor is 0.100000",
					Direction: sdk.ScaleDirectionUp,
				},
			},
			expectedError: nil,
			name:          "scale from 0 with 0 target eg. build queue with 0 running build agents",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 0}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "0"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 5,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 0}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "0"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     0,
					Direction: sdk.ScaleDirectionDown,
					Reason:    "scaling down because factor is 0.000000",
				},
			},
			expectedError: nil,
			name:          "scale to 0 with 0 target eg. idle build agents",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 5.00001}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "5"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 8,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 5.00001}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "5"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "don't scale when the factor change is small",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 5.00001}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "5", "threshold": "0.000001"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 8,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{sdk.TimestampedMetric{Value: 5.00001}},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "5", "threshold": "0.000001"},
					},
				},
				Action: &sdk.ScalingAction{
					Count:     9,
					Reason:    "scaling up because factor is 1.000002",
					Direction: sdk.ScaleDirectionUp,
				},
			},
			expectedError: nil,
			name:          "scale up on small changes if threshold is small",
		},
		{
			inputEval: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{
					sdk.TimestampedMetric{Value: 13, Timestamp: time.Unix(1600000000, 0)},
					sdk.TimestampedMetric{Value: 14, Timestamp: time.Unix(1600000001, 0)},
					sdk.TimestampedMetric{Value: 15, Timestamp: time.Unix(1600000002, 0)},
					sdk.TimestampedMetric{Value: 16, Timestamp: time.Unix(1600000003, 0)},
				},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "16"},
					},
				},
				Action: &sdk.ScalingAction{},
			},
			inputCount: 2,
			expectedResp: &sdk.ScalingCheckEvaluation{
				Metrics: sdk.TimestampedMetrics{
					sdk.TimestampedMetric{Value: 13, Timestamp: time.Unix(1600000000, 0)},
					sdk.TimestampedMetric{Value: 14, Timestamp: time.Unix(1600000001, 0)},
					sdk.TimestampedMetric{Value: 15, Timestamp: time.Unix(1600000002, 0)},
					sdk.TimestampedMetric{Value: 16, Timestamp: time.Unix(1600000003, 0)},
				},
				Check: &sdk.ScalingPolicyCheck{
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: map[string]string{"target": "16"},
					},
				},
				Action: &sdk.ScalingAction{
					Direction: sdk.ScaleDirectionNone,
				},
			},
			expectedError: nil,
			name:          "properly handle multiple input",
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
		inputFactor    float64
		threshold      float64
		expectedOutput sdk.ScaleDirection
	}{
		{inputCount: 0, inputFactor: 1, expectedOutput: sdk.ScaleDirectionUp},
		{inputCount: 5, inputFactor: 1, expectedOutput: sdk.ScaleDirectionNone},
		{inputCount: 4, inputFactor: 0.5, expectedOutput: sdk.ScaleDirectionDown},
		{inputCount: 5, inputFactor: 2, expectedOutput: sdk.ScaleDirectionUp},
		{inputCount: 5, inputFactor: 1.0001, threshold: 0.01, expectedOutput: sdk.ScaleDirectionNone},
		{inputCount: 5, inputFactor: 1.02, threshold: 0.01, expectedOutput: sdk.ScaleDirectionUp},
		{inputCount: 5, inputFactor: 0.99, threshold: 0.01, expectedOutput: sdk.ScaleDirectionNone},
		{inputCount: 5, inputFactor: 0.98, threshold: 0.01, expectedOutput: sdk.ScaleDirectionDown},
	}

	s := &StrategyPlugin{}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, s.calculateDirection(tc.inputCount, tc.inputFactor, tc.threshold))
	}
}
