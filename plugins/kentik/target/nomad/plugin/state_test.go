// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	mock_node "github.com/hashicorp/nomad-autoscaler/mocks/pkg/node"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	statusHandlerInitTimeout = 3 * time.Second
}

func Test_newJobStateHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	nodeStatus := mock_node.NewMockStatus(ctrl)

	handler := func(w http.ResponseWriter, r *http.Request) {
		scaleStatus := `
{
  "JobCreateIndex": 10,
  "JobID": "example",
  "Namespace": "default",
  "JobModifyIndex": 18,
  "JobStopped": false,
  "TaskGroups": {
    "cache": {
      "Desired": 1,
      "Events": null,
      "Healthy": 1,
      "Placed": 1,
      "Running": 0,
      "Unhealthy": 0
    }
  }
}`
		w.Write([]byte(scaleStatus))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	// Create an actual client so we can test it gets set properly.
	cfg := api.DefaultConfig()
	cfg.Address = server.URL
	c, err := api.NewClient(cfg)
	require.NoError(t, err)

	// Create the new handler and perform assertions.
	jsh, err := newJobScaleStatusHandler(c, "default", "test", false, hclog.NewNullLogger(), nodeStatus)
	require.NoError(t, err)

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
			inputJSH:       &jobScaleStatusHandler{scaleStatusError: errors.New("this is an error message")},
			inputGroup:     "test",
			expectedReturn: nil,
			expectedError:  errors.New("this is an error message"),
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
			expectedError:  errors.New("task group \"this-doesnt-exist\" not found"),
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
			tc.inputJSH.logger = hclog.NewNullLogger()
			actualReturn, actualErr := tc.inputJSH.status(tc.inputGroup)
			assert.Equal(t, tc.expectedReturn, actualReturn, tc.name)
			assert.Equal(t, tc.expectedError, actualErr, tc.name)
		})
	}
}

func Test_jobStateHandler_updateStatusState(t *testing.T) {
	jsh := &jobScaleStatusHandler{}
	jsh.initialDone = make(chan bool)

	// Assert that the lastUpdated timestamp is default. This helps confirm it
	// gets updated later in the test.
	assert.Equal(t, int64(0), jsh.lastUpdated)

	// Write our first update.
	jsh.updateStatusState(&api.JobScaleStatusResponse{JobID: "test"}, nil, nil, nil)
	newTimestamp := jsh.lastUpdated
	assert.Equal(t, &api.JobScaleStatusResponse{JobID: "test"}, jsh.scaleStatus)
	assert.Nil(t, jsh.scaleStatusError)
	assert.Greater(t, newTimestamp, int64(0))

	// Write a second update and ensure it is persisted.
	jsh.updateStatusState(nil, nil, errors.New("oh no, something went wrong"), nil)
	assert.Greater(t, jsh.lastUpdated, newTimestamp)
	assert.Equal(t, errors.New("oh no, something went wrong"), jsh.scaleStatusError)
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
