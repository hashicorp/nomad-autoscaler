package status

import (
	"sync"

	"github.com/hashicorp/nomad/api"
)

// State is the interface which encompasses more specific interfaces which
// related more tightly with objects from the Nomad API.
type State interface {
	JobState
}

// Ensure Backend satisfies the JobState interface.
var _ JobState = (*Backend)(nil)

// Backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type Backend struct {

	// jobStateLock is the lock that protects access to the jobState. Using
	// individual locks allows for better, more efficient access control when
	// storing state objects for a number of distinct resources.
	jobStateLock sync.RWMutex
	jobState     map[string]*jobStatus
}

// NewStateBackend returns the Backend implementation of the State interface.
func NewStateBackend() State {
	return &Backend{
		jobState: make(map[string]*jobStatus),
	}
}

type jobStatus struct {
	jobID      string
	stopped    bool
	taskGroups map[string]*api.TaskGroupScaleStatus
}

// DeleteJob satisfies the DeleteJob function on the State interface.
func (b *Backend) DeleteJob(jobID string) {
	b.jobStateLock.Lock()
	defer b.jobStateLock.Unlock()
	delete(b.jobState, jobID)
}

// GetGroup satisfies the GetGroup function on the State interface.
func (b *Backend) GetGroup(jobID, group string) *api.TaskGroupScaleStatus {
	b.jobStateLock.RLock()
	defer b.jobStateLock.RUnlock()

	if val, ok := b.jobState[jobID]; ok {
		return val.taskGroups[group]
	}
	return nil
}

// IsJobStopped satisfies the IsJobStopped function on the State interface.
func (b *Backend) IsJobStopped(jobID string) bool {
	b.jobStateLock.RLock()
	defer b.jobStateLock.RUnlock()

	// Find the real value is we can, otherwise indicate to the caller that the
	// job is stopped.
	if val, ok := b.jobState[jobID]; ok {
		return val.stopped
	}
	return true
}

// SetJob satisfies the SetJob function on the State interface.
func (b *Backend) SetJob(status *api.JobScaleStatusResponse) {
	b.jobStateLock.Lock()
	defer b.jobStateLock.Unlock()

	if _, ok := b.jobState[status.JobID]; !ok {
		b.jobState[status.JobID] = &jobStatus{jobID: status.JobID}
	}

	b.jobState[status.JobID].taskGroups = make(map[string]*api.TaskGroupScaleStatus)

	for name, group := range status.TaskGroups {
		b.jobState[status.JobID].taskGroups[name] = &group
	}
}
