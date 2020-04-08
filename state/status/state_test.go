package status

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestBackend(t *testing.T) {

	// Setup a new backend.
	b := NewStateBackend()
	assert.NotNil(t, b)

	// Write a job status result.
	job1 := generateScaleStatusResponse("job-1", 2)
	b.SetJob(job1)

	// Ensure we can read both groups state from job-1.
	job1Group1 := b.GetGroup("job-1", "job-1-task-group-1")
	job1Group2 := b.GetGroup("job-1", "job-1-task-group-2")
	assert.Equal(t, job1.TaskGroups["job-1-task-group-1"], *job1Group1)
	assert.Equal(t, job1.TaskGroups["job-1-task-group-2"], *job1Group2)

	// Check the status of the job.
	assert.False(t, b.IsJobStopped("job-1"))

	// Write a new job status result.
	job2 := generateScaleStatusResponse("job-2", 1)
	b.SetJob(job2)

	// Read the job out and check its status.
	job2Group1 := b.GetGroup(job2.JobID, "job-2-task-group-1")
	assert.Equal(t, job2.TaskGroups["job-2-task-group-1"], *job2Group1)
	assert.False(t, b.IsJobStopped(job2.JobID))

	// Simulate a scaling update update to job2group1.
	job2.TaskGroups["job-2-task-group-1"].Events[0].Time = uint64(time.Now().UnixNano())
	b.SetJob(job2)
	assert.Equal(t, job2.TaskGroups["job-2-task-group-1"], *job2Group1)
	assert.False(t, b.IsJobStopped(job2.JobID))

	// Delete the job and then ensure it doesn't exist and is reported as stopped.
	b.DeleteJob(job2.JobID)
	assert.True(t, b.IsJobStopped(job2.JobID))
	assert.Nil(t, b.GetGroup(job2.JobID, "job-2-task-group-1"))
}

func generateScaleStatusResponse(jobID string, numTaskGroups int) *api.JobScaleStatusResponse {
	resp := api.JobScaleStatusResponse{
		JobID:      jobID,
		TaskGroups: make(map[string]api.TaskGroupScaleStatus),
	}

	for i := 1; i <= numTaskGroups; i++ {
		resp.TaskGroups[fmt.Sprintf("%s-task-group-%v", jobID, i)] = api.TaskGroupScaleStatus{
			Events: make([]api.ScalingEvent, 1),
		}
	}
	return &resp
}
