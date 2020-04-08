package status

import (
	"github.com/hashicorp/nomad/api"
)

// JobState is the interface which must be met in order to implement the
// autoscalers internal scale status state store which focuses on job scale
// status.
type JobState interface {

	// DeleteJob is used to delete all the stored scaling status state for the
	// specified job.
	DeleteJob(jobID string)

	// GetGroup returns the scale status information for the specified job and
	// group.
	GetGroup(jobID, group string) *api.TaskGroupScaleStatus

	// IsJobStopped returns whether or not the job is in a stopped state.
	IsJobStopped(jobID string) bool

	// SetJob is used to insert the status response object in our internal
	// store. Any existing information should be overwritten as Nomad is the
	// source of truth. This also helps account for Nomad GC events and stops
	// the Autoscaler having to tidy its own state.
	SetJob(status *api.JobScaleStatusResponse)
}
