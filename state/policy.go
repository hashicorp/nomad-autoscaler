package state

import (
	"github.com/hashicorp/nomad-autoscaler/state/policy"
)

// policyUpdateHandler handles all updates from the policy.Watcher and is
// responsible for all safety checks and sanitization before the policy is
// placed into our internal state.
func (h *Handler) policyUpdateHandler() {
	for {
		select {

		// The channel has received a policy which has been determined to
		// have a update. Assign this to `p` and perform the processing
		// required to reflect the change in our state.
		case p := <-h.policyUpdateChan:

			// Protect against nil policies so senders do not have to, keeping
			// this logic in a single place.
			if p == nil {
				break
			}

			jobID := p.Target["Job"]

			// Ensure the scale status watcher is running for the job.
			h.startJobStatusWatcher(jobID)

			// TODO(jrasell) once we have a better method for surfacing errors to the
			//  user, this error should be presented.
			if err := policy.Validate(p); err != nil {
				h.log.Error("failed to validate policy", "error", err, "policy_id", p.ID)
				break
			}

			if p.Policy[policy.KeySource] == nil {
				p.Policy[policy.KeySource] = ""
			}

			autoPolicy := &policy.Policy{
				ID:       p.ID,
				Min:      *p.Min,
				Max:      p.Max,
				Enabled:  *p.Enabled,
				Source:   p.Policy[policy.KeySource].(string),
				Query:    p.Policy[policy.KeyQuery].(string),
				Target:   policy.ParseTarget(p.Policy[policy.KeyTarget]),
				Strategy: policy.ParseStrategy(p.Policy[policy.KeyStrategy]),
			}

			policy.Canonicalize(p, autoPolicy)

			h.PolicyState.Set(jobID, autoPolicy)
			h.log.Info("set policy in state", "policy_id", autoPolicy.ID)

		case <-h.ctx.Done():
			return
		}
	}
}
