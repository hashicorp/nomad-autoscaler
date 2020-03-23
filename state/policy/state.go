package policy

import (
	"sync"
)

// State is the interface which must be met in order to implement the
// autoscalers internal policy state store.
type State interface {

	// List is used to return all the currently stored policies.
	List() map[string]*Policy

	// DeletePolicy is used to delete an individual scaling policy associated
	// to the specified job.
	DeletePolicy(jobID, policyID string)

	// DeleteJobPolicies will delete all the scaling policies associated to the
	// specified job.
	DeleteJobPolicies(jobID string)

	// Set stores the passed policy into the state store. If a policy with the
	// same identifier already exists, this call should perform an overwrite as
	// the internal state should use Nomad as the source of truth.
	Set(jobID string, policy *Policy)
}

// Ensure Backend satisfies the State interface.
var _ State = (*Backend)(nil)

// Backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type Backend struct {
	lock   sync.RWMutex
	state  map[string]*Policy
	jobIDs map[string]map[string]interface{}
}

// NewStateBackend returns the Backend implementation of the State interface.
func NewStateBackend() *Backend {
	return &Backend{
		state:  make(map[string]*Policy),
		jobIDs: make(map[string]map[string]interface{}),
	}
}

// List satisfies the List function on the State interface.
func (b *Backend) List() map[string]*Policy {
	b.lock.RLock()
	policies := b.state
	b.lock.RUnlock()
	return policies
}

// Set satisfies the Set function on the State interface.
func (b *Backend) Set(jobID string, policy *Policy) {
	b.lock.Lock()
	b.state[policy.ID] = policy

	if _, ok := b.jobIDs[jobID]; !ok {
		b.jobIDs[jobID] = make(map[string]interface{})
	}

	b.jobIDs[jobID][policy.ID] = nil
	b.lock.Unlock()
}

// DeletePolicy satisfies the DeletePolicy function on the State interface.
func (b *Backend) DeletePolicy(jobID, policyID string) {
	b.lock.Lock()

	delete(b.jobIDs[jobID], policyID)

	if len(b.jobIDs[jobID]) == 0 {
		delete(b.jobIDs, jobID)
	}

	delete(b.state, policyID)

	b.lock.Unlock()
}

// DeleteJobPolicies satisfies the DeleteJobPolicies function on the State
// interface.
func (b *Backend) DeleteJobPolicies(jobID string) {
	b.lock.Lock()

	policyIDs := b.jobIDs[jobID]
	for id := range policyIDs {
		delete(b.state, id)
	}

	delete(b.jobIDs, jobID)
	b.lock.Unlock()
}
