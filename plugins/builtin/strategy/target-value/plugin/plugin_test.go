package plugin

import (
	"fmt"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
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
		inputReq      strategy.RunRequest
		expectedResp  strategy.RunResponse
		expectedError error
		name          string
	}{
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   nil,
			},
			expectedResp:  strategy.RunResponse{Actions: []strategy.Action{}},
			expectedError: fmt.Errorf("missing required field `target`"),
			name:          "incorrect input config",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "not-the-float-you're-looking-for"},
			},
			expectedResp:  strategy.RunResponse{Actions: []strategy.Action{}},
			expectedError: fmt.Errorf("invalid value for `target`: not-the-float-you're-looking-for (string)"),
			name:          "incorrect input config target value",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "13"},
				Metric:   13,
				Count:    2,
			},
			expectedResp:  strategy.RunResponse{Actions: []strategy.Action{}},
			expectedError: nil,
			name:          "factor equals 1 with non-zero count",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "13"},
				Metric:   26,
				Count:    2,
			},
			expectedResp: strategy.RunResponse{Actions: []strategy.Action{
				{
					Count:  intToPointer(4),
					Reason: "scaling up because factor is 2.000000",
				},
			}},
			expectedError: nil,
			name:          "factor greater than 1 with non-zero count",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "10"},
				Metric:   20,
				Count:    0,
			},
			expectedResp: strategy.RunResponse{Actions: []strategy.Action{
				{
					Count:  intToPointer(2),
					Reason: "scaling up because factor is 2.000000",
				},
			}},
			expectedError: nil,
			name:          "scale from 0 with non-zero target",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "10"},
				Metric:   9,
				Count:    1,
			},
			expectedResp:  strategy.RunResponse{Actions: []strategy.Action{}},
			expectedError: nil,
			name:          "no scaling based on small target/metric difference",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "0"},
				Metric:   1,
				Count:    0,
			},
			expectedResp: strategy.RunResponse{Actions: []strategy.Action{
				{
					Count:  intToPointer(1),
					Reason: "scaling up because factor is 1.000000",
				},
			}},
			expectedError: nil,
			name:          "scale from 0 with 0 target eg. build queue with 0 running build agents",
		},
		{
			inputReq: strategy.RunRequest{
				PolicyID: "test-policy",
				Config:   map[string]string{"target": "0"},
				Metric:   0,
				Count:    5,
			},
			expectedResp: strategy.RunResponse{Actions: []strategy.Action{
				{
					Count:  intToPointer(0),
					Reason: "scaling down because factor is 0.000000",
				},
			}},
			expectedError: nil,
			name:          "scale to 0 with 0 target eg. idle build agents",
		},
	}

	s := &StrategyPlugin{logger: hclog.NewNullLogger()}

	for _, tc := range testCases {
		actualResp, actualError := s.Run(tc.inputReq)
		assert.Equal(t, tc.expectedResp, actualResp)
		assert.Equal(t, tc.expectedError, actualError)
	}
}

func TestStrategyPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputCount     int64
		inputFactor    float64
		expectedOutput string
	}{
		{inputCount: 0, inputFactor: 1, expectedOutput: "up"},
		{inputCount: 5, inputFactor: 1, expectedOutput: ""},
		{inputCount: 4, inputFactor: 0.5, expectedOutput: "down"},
		{inputCount: 5, inputFactor: 2, expectedOutput: "up"},
	}

	s := &StrategyPlugin{}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, s.calculateDirection(tc.inputCount, tc.inputFactor))
	}
}

func intToPointer(i int64) *int64 { return &i }
