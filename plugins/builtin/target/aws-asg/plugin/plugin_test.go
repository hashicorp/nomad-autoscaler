package plugin

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputAsgDesired      int64
		inputStrategyDesired int64
		expectedOutputNum    int64
		expectedOutputString string
		name                 string
	}{
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 11,
			expectedOutputNum:    11,
			expectedOutputString: "out",
			name:                 "scale out desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 9,
			expectedOutputNum:    1,
			expectedOutputString: "in",
			name:                 "scale in desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 10,
			expectedOutputNum:    0,
			expectedOutputString: "",
			name:                 "scale not desired",
		},
	}

	tp := TargetPlugin{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNum, actualString := tp.calculateDirection(tc.inputAsgDesired, tc.inputStrategyDesired)
			assert.Equal(t, tc.expectedOutputNum, actualNum, tc.name)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
		})
	}
}

func Test_processLastActivity(t *testing.T) {

	testTime := time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC)

	testCases := []struct {
		inputActivity  autoscaling.Activity
		inputStatus    *sdk.TargetStatus
		expectedStatus *sdk.TargetStatus
		name           string
	}{
		{
			inputActivity: autoscaling.Activity{
				Progress: int64ToPtr(75),
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: false,
				Count: 1,
				Meta:  map[string]string{},
			},
			name: "latest activity still in progress",
		},
		{
			inputActivity: autoscaling.Activity{
				Progress: int64ToPtr(100),
				EndTime:  &testTime,
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta: map[string]string{
					"nomad_autoscaler.last_event": "1586765040000000000",
				},
			},
			name: "latest activity completed",
		},
		{
			inputActivity: autoscaling.Activity{},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			name: "latest activity all nils",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processLastActivity(tc.inputActivity, tc.inputStatus)
			assert.Equal(t, tc.expectedStatus, tc.inputStatus, tc.name)
		})
	}
}

func int64ToPtr(v int64) *int64 {
	return &v
}
