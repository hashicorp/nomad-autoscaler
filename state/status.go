package state

import (
	"github.com/hashicorp/nomad-autoscaler/state/status"
)

// jobStatusUpdateHandler handles all updates from the status.Watcher and is
// responsible for all safety checks and sanitization before the status is
// placed into our internal state.
func (h *Handler) jobStatusUpdateHandler() {
	for {
		select {

		// Receive a job status update from the channel.
		case update := <-h.statusUpdateChan:

			// Protect against nil policies so senders do not have to.
			if update == nil {
				break
			}

			// Set the update within our state store.
			h.statusState.SetJob(update)
			h.log.Debug("set scale status in state", "job_id", update.JobID)

		case <-h.ctx.Done():
			return
		}
	}
}

// startJobStatusWatcher will start a new job scale status blocking watcher if
// needed. It will also wait for the first run of the watcher to process the
// API data, which is required before processing a scaling policy.
func (h *Handler) startJobStatusWatcher(jobID string) {
	h.statusWatcherHandlerLock.Lock()
	defer h.statusWatcherHandlerLock.Unlock()

	if handle, ok := h.statusWatcherHandlers[jobID]; ok {
		h.log.Debug("found existing handler for job scale status", "job_id", jobID)

		// It is possible the existing handle was found, but the watcher wasn't
		// running. This is usually caused by job purging and the fact the
		// handle has not been GC'd yet.
		if !handle.IsRunning {
			h.log.Debug("starting existing job scale status handle", "job_id", jobID)
			go h.statusWatcherHandlers[jobID].Start(h.statusUpdateChan)
		}
		return
	}
	h.log.Debug("starting new handler for job scale status", "job_id", jobID)

	// Setup the new handler.
	h.statusWatcherHandlers[jobID] = status.NewWatcher(h.log, jobID, h.nomad)
	go h.statusWatcherHandlers[jobID].Start(h.statusUpdateChan)
}

// removeJobStatusHandler will remove the scale status handler for the job from
// the state handlers mapping.
//
// TODO(jrasell) this should shutdown the blocking query; currently the query
//  will just reach the timeout.
func (h *Handler) removeJobStatusHandler(jobID string) {
	h.statusWatcherHandlerLock.Lock()
	defer h.statusWatcherHandlerLock.Unlock()

	// Delete the handle.
	delete(h.statusWatcherHandlers, jobID)
	h.log.Debug("deleted scale status watcher handle", "job_id", jobID)
}
