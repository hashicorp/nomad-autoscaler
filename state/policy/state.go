package policy

import (
	"sync"

	"github.com/hashicorp/nomad/api"
)

// State is the interface which must be met in order to implement the
// autoscalers internal policy state store.
type State interface {

	// List is used to return all the currently stored policies.
	List() map[string]*Policy

	// DeletePolicies will delete all the scaling policies associated to the
	// specified resource.
	DeletePolicies(resourceID string)

	// ReconcilePolicies compares the input policy list against the internal
	// store. Any policies that are found within the store, but are not present
	// in the input list are removed.
	ReconcilePolicies(policies []*api.ScalingPolicyListStub)

	// Set stores the passed policy into the state store. If a policy with the
	// same identifier already exists, this call should perform an overwrite as
	// the internal state should use Nomad as the source of truth.
	Set(resourceID string, policy *Policy)
}

// Ensure Backend satisfies the State interface.
var _ State = (*backend)(nil)

// Backend is currently the only implementation of the State interface and
// provides and in-memory state store with locking safety.
type backend struct {

	// lock is the mutex that should be used when interacting with either of
	// the below maps.
	lock sync.RWMutex

	// state contains all the known scaling policy which the autoscaler is
	// responsible for evaluating. The map key is the ID of the policy object
	// as determined by the policy source.
	state map[string]*Policy

	// resourceIDs is a helper map which translates an individual resource to
	// many scaling policies. This is useful for deleting all the policies
	// associated to a resource when a resource can have > 1 scaling policy. It
	// also provides use when needing to delete policies based on a resource ID
	// rather than the policy ID.
	resourceIDs map[string]map[string]interface{}
}

// NewStateBackend returns the Backend implementation of the State interface.
func NewStateBackend() State {
	return &backend{
		state:       make(map[string]*Policy),
		resourceIDs: make(map[string]map[string]interface{}),
	}
}

// List satisfies the List function on the State interface.
func (b *backend) List() map[string]*Policy {
	b.lock.RLock()
	policies := b.state
	b.lock.RUnlock()
	return policies
}

// Set satisfies the Set function on the State interface.
func (b *backend) Set(resourceID string, policy *Policy) {
	b.lock.Lock()
	b.state[policy.ID] = policy

	if _, ok := b.resourceIDs[resourceID]; !ok {
		b.resourceIDs[resourceID] = make(map[string]interface{})
	}

	b.resourceIDs[resourceID][policy.ID] = nil
	b.lock.Unlock()
}

// DeletePolicies satisfies the DeletePolicies function on the State interface.
func (b *backend) DeletePolicies(resourceID string) {
	b.lock.Lock()

	policyIDs := b.resourceIDs[resourceID]
	for id := range policyIDs {
		delete(b.state, id)
	}

	delete(b.resourceIDs, resourceID)
	b.lock.Unlock()
}

// ReconcilePolicies satisfies the ReconcilePolicies function on the State
// interface.
func (b *backend) ReconcilePolicies(policies []*api.ScalingPolicyListStub) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for k, v := range b.state {

		var found bool

		for _, listPolicy := range policies {
			if k == listPolicy.ID {
				found = true
				break
			}
		}

		if !found {
			if id, ok := v.Target.Config["job_id"]; ok {
				b.deletePolicy(id, k)
			}
			delete(b.state, k)
		}
	}
}

func (b *backend) deletePolicy(resourceID, policyID string) {
	delete(b.resourceIDs[resourceID], policyID)

	if len(b.resourceIDs[resourceID]) == 0 {
		delete(b.resourceIDs, resourceID)
	}

	delete(b.state, policyID)
}
