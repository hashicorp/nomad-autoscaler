package nomad

import (
	"fmt"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_newJobStateHandler(t *testing.T) {

	// Create an actual client so we can test it gets set properly.
	c, err := api.NewClient(api.DefaultConfig())
	assert.Nil(t, err)

	// Create the new handler and perform assertions.
	jsh := newJobScaleStatusHandler(c, "test", hclog.NewNullLogger())
	assert.NotNil(t, jsh.client)
	assert.Equal(t, "test", jsh.jobID)
	assert.NotNil(t, jsh.initialDone)
	assert.NotNil(t, jsh.client)
}

func Test_jobStateHandler_status(t *testing.T) {
	testCases := []struct {
		inputJSH       *jobScaleStatusHandler
		inputGroup     string
		expectedReturn *sdk.TargetStatus
		expectedError  error
		name           string
	}{
		{
			inputJSH:       &jobScaleStatusHandler{scaleStatusError: fmt.Errorf("this is an error message")},
			inputGroup:     "test",
			expectedReturn: nil,
			expectedError:  fmt.Errorf("this is an error message"),
			name:           "job status response currently in error",
		},
		{
			inputJSH:       &jobScaleStatusHandler{},
			inputGroup:     "test",
			expectedReturn: nil,
			expectedError:  nil,
			name:           "job no longer running on cluster",
		},
		{
			inputJSH: &jobScaleStatusHandler{
				scaleStatus: &api.JobScaleStatusResponse{
					TaskGroups: map[string]api.TaskGroupScaleStatus{},
				},
			},
			inputGroup:     "this-doesnt-exist",
			expectedReturn: nil,
			expectedError:  fmt.Errorf("task group \"this-doesnt-exist\" not found"),
			name:           "job group not found within scale status task groups",
		},
		{
			inputJSH: &jobScaleStatusHandler{
				jobID: "cant-think-of-a-funny-name",
				scaleStatus: &api.JobScaleStatusResponse{
					JobStopped: false,
					TaskGroups: map[string]api.TaskGroupScaleStatus{
						"this-does-exist": {Running: 7},
					},
				},
			},
			inputGroup: "this-does-exist",
			expectedReturn: &sdk.TargetStatus{
				Ready: true,
				Count: 7,
				Meta: map[string]string{
					"nomad_autoscaler.target.nomad.cant-think-of-a-funny-name.stopped": "false",
				},
			},
			expectedError: nil,
			name:          "job group found within scale status task groups and job is running",
		},
		{
			inputJSH: &jobScaleStatusHandler{
				jobID: "cant-think-of-a-funny-name",
				scaleStatus: &api.JobScaleStatusResponse{
					JobStopped: true,
					TaskGroups: map[string]api.TaskGroupScaleStatus{
						"this-does-exist": {Running: 7},
					},
				},
			},
			inputGroup: "this-does-exist",
			expectedReturn: &sdk.TargetStatus{
				Ready: false,
				Count: 7,
				Meta: map[string]string{
					"nomad_autoscaler.target.nomad.cant-think-of-a-funny-name.stopped": "true",
				},
			},
			expectedError: nil,
			name:          "job group found within scale status task groups and job is not running",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualReturn, actualErr := tc.inputJSH.status(tc.inputGroup)
			assert.Equal(t, tc.expectedReturn, actualReturn, tc.name)
			assert.Equal(t, tc.expectedError, actualErr, tc.name)
		})
	}
}

func Test_jobStateHandler_updateStatusState(t *testing.T) {
	jsh := &jobScaleStatusHandler{}

	// Assert that the lastUpdated timestamp is default. This helps confirm it
	// gets updated later in the test.
	assert.Equal(t, int64(0), jsh.lastUpdated)

	// Write our first update.
	jsh.updateStatusState(&api.JobScaleStatusResponse{JobID: "test"}, nil)
	newTimestamp := jsh.lastUpdated
	assert.Equal(t, &api.JobScaleStatusResponse{JobID: "test"}, jsh.scaleStatus)
	assert.Nil(t, jsh.scaleStatusError)
	assert.Greater(t, newTimestamp, int64(0))

	// Write a second update and ensure it is persisted.
	jsh.updateStatusState(nil, fmt.Errorf("oh no, something went wrong"))
	assert.Greater(t, jsh.lastUpdated, newTimestamp)
	assert.Equal(t, fmt.Errorf("oh no, something went wrong"), jsh.scaleStatusError)
	assert.Nil(t, jsh.scaleStatus)
}

func Test_jobStateHandler_stop(t *testing.T) {
	jsh := &jobScaleStatusHandler{}

	// Assert that the lastUpdated timestamp is default. This helps confirm it
	// gets updated later in the test.
	assert.Equal(t, int64(0), jsh.lastUpdated)

	// Set some data that will be overwritten by stop().
	jsh.isRunning = true
	jsh.scaleStatus = &api.JobScaleStatusResponse{JobID: "test"}

	// Call stop and make assertions.
	jsh.setStopState()
	assert.False(t, jsh.isRunning)
	assert.Nil(t, jsh.scaleStatus)
	assert.Nil(t, jsh.scaleStatusError)
	assert.Greater(t, jsh.lastUpdated, int64(0))
}
