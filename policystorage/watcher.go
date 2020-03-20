package policystorage

import (
	"time"

	"github.com/hashicorp/nomad/api"
)

// StartPolicyWatcher triggers the running on the scaling policy watcher, which
// uses blocking queries to monitor the Nomad API for changes in the stored
// scaling polices. Changes the occur, are processed and then store within the
// internal state for fast access.
func (s *Store) StartPolicyWatcher() {
	var maxFound uint64

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		// Perform a blocking query on the Nomad API that returns a stub list
		// of scaling policies. If we get an errors at this point, we should
		// sleep and try again.
		//
		// TODO(jrasell) in the future maybe use a better method than sleep.
		policies, meta, err := s.nomad.Scaling().ListPolicies(q)
		if err != nil {
			s.log.Error("failed to call the Nomad list policies API", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !indexHasChange(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Iterate all policies in the list.
		for _, policy := range policies {

			// If the index on the individual policy is not great than our last
			// seen, look at the next policy. If it is great, then move forward
			// and process the policy.
			if !indexHasChange(policy.ModifyIndex, s.lastChangeIndex) {
				continue
			}

			// Perform a read on the policy to get all the information.
			p, _, err := s.nomad.Scaling().GetPolicy(policy.ID, nil)
			if err != nil {
				s.log.Error("failed call the Nomad read policy API",
					"error", err, "policy-id", policy.ID)
				continue
			}
			s.handlePolicyUpdate(p)

			// Update our currently recorded maxFound index.
			maxFound = findMaxFound(policy.ModifyIndex, maxFound)
		}

		// Is there some other way to run this? This is needed to handle job
		// registration updates where a user has removed a scaling block from a
		// group.
		s.handlePolicyCleanup(policies)

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex
		s.lastChangeIndex = maxFound
	}
}

// handlePolicyUpdate is responsible for taking a policy from the Nomad API,
// turning this into an internally usable object and storing it within the
// state store for use.
func (s *Store) handlePolicyUpdate(policy *api.ScalingPolicy) {
	// TODO(jrasell) once we have a better method for surfacing errors to the
	//  user, this error should be presented.
	if err := validate(policy); err != nil {
		s.log.Error("failed to validate policy", "error", err, "policy-id", policy.ID)
		return
	}

	if policy.Policy[policyKeySource] == nil {
		policy.Policy[policyKeySource] = ""
	}

	autoPolicy := &Policy{
		ID:       policy.ID,
		Min:      policy.Min,
		Max:      policy.Max,
		Source:   policy.Policy[policyKeySource].(string),
		Query:    policy.Policy[policyKeyQuery].(string),
		Target:   parseTarget(policy.Policy[policyKeyTarget]),
		Strategy: parseStrategy(policy.Policy[policyKeyStrategy]),
	}

	canonicalize(policy, autoPolicy)

	s.State.set(autoPolicy)
	s.log.Info("set policy in state", "policy-id", policy.ID)
}

// handlePolicyCleanup is used to clean-up the autoscalers local policy state
// storage and bring it inline with what is stored in Nomad.
func (s *Store) handlePolicyCleanup(policyList []*api.ScalingPolicyListStub) {
	current := s.State.List()

	// Create a map and populate with IDs based on the current state list.
	ids := make(map[string]interface{})
	for id := range current {
		ids[id] = nil
	}

	// Iterate the Nomad returned policy list, deleting each ID from our temp.
	// map.
	for _, nomadPolicy := range policyList {
		delete(ids, nomadPolicy.ID)
	}

	// If there are remaining IDs in the temp. map; delete them from the store
	// as they no longer exist in Nomad.
	if len(ids) > 0 {
		for id := range ids {
			s.State.delete(id)
			s.log.Info("deleted policy from state", "policy-id", id)
		}
	}
}

// indexHasChange is used to check whether a returned blocking query has an
// updated index, compared to a tracked value.
func indexHasChange(new, old uint64) bool {
	if new <= old {
		return false
	}
	return true
}

// maxFound is used to determine which value passed is the greatest. This is
// used to track the most recently found highest index value.
func findMaxFound(new, old uint64) uint64 {
	if new <= old {
		return old
	}
	return new
}
