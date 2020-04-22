package state

import "time"

const (
	// garbageCollectionInterval is the interval in seconds at which the
	// garbage collection process is run.
	garbageCollectionInterval = 60

	// garbageCollectionThreshold in nanoseconds, defines the threshold at
	// which internal status state is considered to be ready for purging.
	// 14400000000000 nanoseconds represents 4hrs.
	garbageCollectionThreshold = 14400000000000
)

// garbageCollectStateLoop runs the loop which triggers the garbage collector
// at a specified interval.
func (h *Handler) garbageCollectStateLoop() {

	// Setup the ticker. The interval is currently an internal const, but could
	// be exposed at a CLI variable if needed in the future.
	ticker := time.NewTicker(garbageCollectionInterval * time.Second)

	for {
		select {
		case <-ticker.C:
			h.log.Debug("initiating state garbage collection run")

			// Perform the status state GC run, collecting a list of removed
			// job IDs.
			deleted := h.statusState.GarbageCollect(garbageCollectionThreshold)

			// Iterate through the IDs, removing any handler level objects
			// or any remaining policies which have ties to the ID. The policy
			// should already be removed but its a noop call so doesn't hurt to
			// be sure.
			for _, jobID := range deleted {
				h.removeJobStatusHandler(jobID)
				h.PolicyState.DeletePolicies(jobID)
				h.log.Debug("performed garbage collection", "job_id", jobID)
			}

		case <-h.ctx.Done():
			return
		}
	}
}

// policyReconciliationHandler is a handler task which is responsible for
// receiving updates from the policyReconcileChan and triggering the policy
// state reconcile handler.
func (h *Handler) policyReconciliationHandler() {
	for {
		select {
		case p := <-h.policyReconcileChan:

			// Protect against receiving nil from the channel. Do not perform a
			// length check here. If the list has a length of zero this still
			// needs to be sent for reconciliation.
			if p == nil {
				continue
			}

			// Log a nice friendly message and perform the reconciliation.
			h.log.Debug("triggering reconciliation of scaling policies")
			h.PolicyState.ReconcilePolicies(p)

		case <-h.ctx.Done():
			return
		}
	}
}
