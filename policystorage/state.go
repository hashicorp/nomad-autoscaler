package policystorage

import (
	"sync"
)

// State is the interface which must be met in order to implement the
// autoscalers internal policy state store.
type State interface {

	// List is used to return all the currently stored policies.
	List() map[string]*Policy

	// delete performs a delete on the passed policy identifier from the state
	// store. If the policy is not stored in state, the call should be no-op.
	delete(id string)

	// set stores the passed policy into the state store. If a policy with the
	// same identifier already exists, this call should perform an overwrite.
	set(policy *Policy)
}

// Ensure Backend satisfies the State interface.
var _ State = (*Backend)(nil)

// Backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type Backend struct {
	lock  sync.RWMutex
	state map[string]*Policy
}

// newStateBackend returns the Backend implementation of the State interface.
func newStateBackend() *Backend {
	return &Backend{
		state: make(map[string]*Policy),
	}
}

// List satisfies the List function on the State interface.
func (b *Backend) List() map[string]*Policy {
	b.lock.RLock()
	policies := b.state
	b.lock.RUnlock()
	return policies
}

// set satisfies the set function on the State interface.
func (b *Backend) set(policy *Policy) {
	b.lock.Lock()
	b.state[policy.ID] = policy
	b.lock.Unlock()
}

// delete satisfies the delete function on the State interface.
func (b *Backend) delete(id string) {
	b.lock.Lock()
	delete(b.state, id)
	b.lock.Unlock()
}
