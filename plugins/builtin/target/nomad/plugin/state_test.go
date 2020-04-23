package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_newJobStateHandler(t *testing.T) {

	// Create an actual client so we can test it gets set properly.
	c, err := api.NewClient(api.DefaultConfig())
	assert.Nil(t, err)

	// Create the new handler and perform assertions.
	jsh := newJobStateHandler(c, "test", hclog.NewNullLogger())
	assert.NotNil(t, jsh.client)
	assert.Equal(t, "test", jsh.jobID)
	assert.NotNil(t, jsh.initialDone)
	assert.NotNil(t, jsh.client)
}

func Test_jobStateHandler_updateStatusState(t *testing.T) {
	jsh := &jobStateHandler{}

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
	jsh := &jobStateHandler{}

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
	assert.Greater(t, jsh.lastUpdated, int64(0))
}
