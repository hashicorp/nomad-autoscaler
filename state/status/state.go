package status

import (
	"sync"

	"github.com/hashicorp/nomad/api"
)

// State is the interface which must be met in order to implement the
// autoscalers internal scale status state store.
type State interface {

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

// Ensure Backend satisfies the State interface.
var _ State = (*Backend)(nil)

type jobStatus struct {
	jobID      string
	stopped    bool
	taskGroups map[string]*api.TaskGroupScaleStatus
}

// Backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type Backend struct {
	lock  sync.RWMutex
	state map[string]*jobStatus
}

// NewStateBackend returns the Backend implementation of the State interface.
func NewStateBackend() *Backend {
	return &Backend{
		state: make(map[string]*jobStatus),
	}
}

// DeleteJob satisfies the DeleteJob function on the State interface.
func (b *Backend) DeleteJob(jobID string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.state, jobID)
}

// GetGroup satisfies the GetGroup function on the State interface.
func (b *Backend) GetGroup(jobID, group string) *api.TaskGroupScaleStatus {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if val, ok := b.state[jobID]; ok {
		return val.taskGroups[group]
	}
	return nil
}

// IsJobStopped satisfies the IsJobStopped function on the State interface.
func (b *Backend) IsJobStopped(jobID string) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()

	// Find the real value is we can, otherwise indicate to the caller that the
	// job is stopped.
	if val, ok := b.state[jobID]; ok {
		return val.stopped
	}
	return true
}

// SetJob satisfies the SetJob function on the State interface.
func (b *Backend) SetJob(status *api.JobScaleStatusResponse) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.state[status.JobID]; !ok {
		b.state[status.JobID] = &jobStatus{jobID: status.JobID}
	}

	b.state[status.JobID].taskGroups = make(map[string]*api.TaskGroupScaleStatus)

	for name, group := range status.TaskGroups {
		b.state[status.JobID].taskGroups[name] = &group
	}
}
