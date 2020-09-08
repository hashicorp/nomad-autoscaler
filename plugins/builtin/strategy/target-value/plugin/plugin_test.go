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
	expectedOutput := &base.PluginInfo{Name: "target-value", PluginType: "strategy"}
	actualOutput, err := s.PluginInfo()
	assert.Nil(t, err)
	assert.Equal(t, expectedOutput, actualOutput)
}

func TestStrategyPlugin_Run(t *testing.T) {
	testCases := []struct {
		inputReq      sdk.StrategyRunReq
		expectedResp  sdk.ScalingAction
		expectedError error
		name          string
	}{
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   nil,
			},
			expectedResp:  sdk.ScalingAction{},
			expectedError: fmt.Errorf("missing required field `target`"),
			name:          "incorrect input config",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "not-the-float-you're-looking-for"},
			},
			expectedResp:  sdk.ScalingAction{},
			expectedError: fmt.Errorf("invalid value for `target`: not-the-float-you're-looking-for (string)"),
			name:          "incorrect input config target value",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "0", "threshold": "not-the-float-you're-looking-for"},
			},
			expectedResp:  sdk.ScalingAction{},
			expectedError: fmt.Errorf("invalid value for `threshold`: not-the-float-you're-looking-for (string)"),
			name:          "incorrect input config target value",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "13"},
				Metric:   13,
				Count:    2,
			},
			expectedResp:  sdk.ScalingAction{Direction: sdk.ScaleDirectionNone},
			expectedError: nil,
			name:          "factor equals 1 with non-zero count",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "13"},
				Metric:   26,
				Count:    2,
			},
			expectedResp: sdk.ScalingAction{
				Count:     4,
				Direction: sdk.ScaleDirectionUp,
				Reason:    "scaling up because factor is 2.000000",
			},
			expectedError: nil,
			name:          "factor greater than 1 with non-zero count",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "10"},
				Metric:   20,
				Count:    0,
			},
			expectedResp: sdk.ScalingAction{
				Count:     2,
				Direction: sdk.ScaleDirectionUp,
				Reason:    "scaling up because factor is 2.000000",
			},
			expectedError: nil,
			name:          "scale from 0 with non-zero target",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "10"},
				Metric:   9,
				Count:    1,
			},
			expectedResp:  sdk.ScalingAction{Direction: sdk.ScaleDirectionNone},
			expectedError: nil,
			name:          "no scaling based on small target/metric difference",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "0"},
				Metric:   1,
				Count:    0,
			},
			expectedResp: sdk.ScalingAction{
				Count:     1,
				Direction: sdk.ScaleDirectionUp,
				Reason:    "scaling up because factor is 1.000000",
			},
			expectedError: nil,
			name:          "scale from 0 with 0 target eg. build queue with 0 running build agents",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "0"},
				Metric:   0,
				Count:    5,
			},
			expectedResp: sdk.ScalingAction{
				Count:     0,
				Direction: sdk.ScaleDirectionDown,
				Reason:    "scaling down because factor is 0.000000",
			},
			expectedError: nil,
			name:          "scale to 0 with 0 target eg. idle build agents",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "5"},
				Metric:   5.00001,
				Count:    8,
			},
			expectedResp:  sdk.ScalingAction{Direction: sdk.ScaleDirectionNone},
			expectedError: nil,
			name:          "don't scale when the factor change is small",
		},
		{
			inputReq: sdk.StrategyRunReq{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "5", "threshold": "0.000001"},
				Metric:   5.00001,
				Count:    8,
			},
			expectedResp: sdk.ScalingAction{
				Count:     9,
				Direction: sdk.ScaleDirectionUp,
				Reason:    "scaling up because factor is 1.000002",
			},
			expectedError: nil,
			name:          "scale up on small changes if threshold is small",
		},
	}

	s := &StrategyPlugin{logger: hclog.NewNullLogger()}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResp, actualError := s.Run(tc.inputReq)
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
