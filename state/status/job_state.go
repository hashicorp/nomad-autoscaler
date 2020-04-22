package status

import (
	"github.com/hashicorp/nomad/api"
)

// JobState is the interface which must be met in order to implement the
// autoscalers internal scale status state store which focuses on job scale
// status.
type JobState interface {

	// GarbageCollect is used to perform job status state garbage collection.
	// The passed threshold (nanoseconds) is compared against the entries last
	// update time to decide whether the object should be removed. A list of
	// removed job IDs should then be returned so that any follow up cleanup
	// can be performed.
	GarbageCollect(threshold int64) []string

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
