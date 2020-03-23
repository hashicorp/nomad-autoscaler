package state

import (
	"github.com/hashicorp/nomad-autoscaler/state/policy"
)

// policyUpdateHandler handles all updates from the policy.Watcher and is
// responsible for all safety checks and sanitization before the policy is
// placed into our internal state.
func (h *Handler) policyUpdateHandler() {
	select {
	case p := <-h.policyUpdateChan:

		jobID := p.Target["Job"]

		// Ensure the scale status watcher is running for the job.
		h.startStatusWatcher(jobID)

		// If the job is stopped, we don't need to work on storing the policy
		// and should exit.
		if h.statusState.IsJobStopped(jobID) {
			return
		}

		// TODO(jrasell) once we have a better method for surfacing errors to the
		//  user, this error should be presented.
		if err := policy.Validate(p); err != nil {
			h.log.Error("failed to validate policy", "error", err, "policy-id", p.ID)
			return
		}

		if p.Policy[policy.KeySource] == nil {
			p.Policy[policy.KeySource] = ""
		}

		autoPolicy := &policy.Policy{
			ID:       p.ID,
			Min:      *p.Min,
			Max:      p.Max,
			Source:   p.Policy[policy.KeySource].(string),
			Query:    p.Policy[policy.KeyQuery].(string),
			Target:   policy.ParseTarget(p.Policy[policy.KeyTarget]),
			Strategy: policy.ParseStrategy(p.Policy[policy.KeyStrategy]),
		}

		policy.Canonicalize(p, autoPolicy)

		h.PolicyState.Set(jobID, autoPolicy)
		h.log.Info("set policy in state", "policy-id", autoPolicy.ID)

	case <-h.ctx.Done():
		return
	}
}
