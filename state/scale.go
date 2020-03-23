package state

import (
	"github.com/hashicorp/nomad-autoscaler/state/status"
)

// statusUpdateHandler handles all updates from the status.Watcher and is
// responsible for all safety checks and sanitization before the status object
// is placed into our internal state.
func (h *Handler) statusUpdateHandler() {
	select {
	case s := <-h.statusUpdateChan:

		// If the job is stopped, remove the scaling policies, remove the scale
		// status handler and delete the scale status state. Even if the job is
		// started again, the Autoscaler will gather all the information it
		// needs so we can clear everything out.
		if s.JobStopped {
			h.PolicyState.DeleteJobPolicies(s.JobID)
			h.removeStatusHandler(s.JobID)
			h.statusState.DeleteJob(s.JobID)
			h.log.Info("job is stopped, removed internal state", "job-id", s.JobID)
		} else {
			// Store the new scale status information and move along.
			h.statusState.SetJob(s)
			h.log.Info("set scale status in state", "job-id", s.JobID)
		}

	case <-h.ctx.Done():
		return
	}
}

// startStatusWatcher will start a new job scale status blocking watcher if
// needed. It will also wait for the first run of the watcher to process the
// API data, which is required before processing a scaling policy.
func (h *Handler) startStatusWatcher(jobID string) {
	h.statusWatcherHandlerLock.Lock()
	defer h.statusWatcherHandlerLock.Unlock()

	if _, ok := h.statusWatcherHandlers[jobID]; ok {
		h.log.Trace("found existing handler for job scale status", "job-id", jobID)
		return
	}
	h.log.Debug("starting new handler for job scale status", "job-id", jobID)

	// Setup the new handler, start it and wait for its initial run to finish.
	h.statusWatcherHandlers[jobID] = status.NewWatcher(h.log, jobID, h.nomad, h.statusUpdateChan)
	go h.statusWatcherHandlers[jobID].Start()
	<-h.statusWatcherHandlers[jobID].InitialLoad
}

// removeStatusHandler will remove the scale status handler for the job from
// the state handlers mapping.
//
// TODO(jrasell) this should shutdown the blocking query; currently the query
//  will just reach the timeout.
func (h *Handler) removeStatusHandler(jobID string) {
	h.statusWatcherHandlerLock.Lock()
	defer h.statusWatcherHandlerLock.Unlock()

	// Delete the handle.
	delete(h.statusWatcherHandlers, jobID)
	h.log.Trace("deleted scale status watcher handle", "job-id", jobID)
}
