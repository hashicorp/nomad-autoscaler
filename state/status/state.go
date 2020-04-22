package status

import (
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
)

// State is the interface which encompasses more specific interfaces which
// relates more tightly with objects from the Nomad API.
type State interface {
	JobState
}

// Ensure Backend satisfies the JobState interface.
var _ JobState = (*backend)(nil)

// backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type backend struct {
	jobState map[string]*jobStatus
	lock     sync.RWMutex
}

// NewStateBackend returns the Backend implementation of the State interface.
func NewStateBackend() State {
	return &backend{
		jobState: make(map[string]*jobStatus),
	}
}

type jobStatus struct {
	jobID      string
	stopped    bool
	taskGroups map[string]*api.TaskGroupScaleStatus
	lastUpdate int64
}

// GetGroup satisfies the GetGroup function on the State interface.
func (b *backend) GetGroup(jobID, group string) *api.TaskGroupScaleStatus {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if val, ok := b.jobState[jobID]; ok {
		return val.taskGroups[group]
	}
	return nil
}

// IsJobStopped satisfies the IsJobStopped function on the State interface.
func (b *backend) IsJobStopped(jobID string) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()

	// Find the real value is we can, otherwise indicate to the caller that the
	// job is stopped.
	if val, ok := b.jobState[jobID]; ok {
		return val.stopped
	}
	return true
}

// SetJob satisfies the SetJob function on the State interface.
func (b *backend) SetJob(status *api.JobScaleStatusResponse) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.jobState[status.JobID]; !ok {
		b.jobState[status.JobID] = &jobStatus{jobID: status.JobID}
	}

	b.jobState[status.JobID].stopped = status.JobStopped
	b.jobState[status.JobID].taskGroups = make(map[string]*api.TaskGroupScaleStatus)
	b.jobState[status.JobID].lastUpdate = time.Now().UnixNano()

	for name, group := range status.TaskGroups {
		b.jobState[status.JobID].taskGroups[name] = &group
	}
}

// GarbageCollect satisfies the GarbageCollect function on the State interface.
func (b *backend) GarbageCollect(threshold int64) []string {
	b.lock.Lock()
	defer b.lock.Unlock()

	gc := time.Now().UTC().UnixNano() - threshold

	d := []string{}

	for job, status := range b.jobState {

		if status.lastUpdate < gc && status.stopped {
			d = append(d, job)
			delete(b.jobState, job)
		}
	}
	return d
}
