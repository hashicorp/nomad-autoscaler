package status

import (
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad/api"
)

type Watcher struct {
	log             hclog.Logger
	jobID           string
	nomad           *api.Client
	lastChangeIndex uint64
	updateChan      chan *api.JobScaleStatusResponse
	IsRunning       bool
}

// NewWatcher creates a new scale status watcher, using blocking queries to
// monitor changes from Nomad.
func NewWatcher(log hclog.Logger, jobID string, nomad *api.Client) *Watcher {
	return &Watcher{
		jobID: jobID,
		log:   log.Named("status_watcher_" + jobID),
		nomad: nomad,
	}
}

// Start starts the query loop that performs a blocking query on the scaling
// status endpoint of a specific job.
func (w *Watcher) Start(updateChan chan *api.JobScaleStatusResponse) {

	// Log that the watcher is starting, and report this.
	w.log.Debug("starting status blocking query watcher")
	w.IsRunning = true

	// Store the update channel.
	w.updateChan = updateChan

	var maxFound uint64

	q := &api.QueryOptions{WaitTime: 1 * time.Minute, WaitIndex: 1}

	for {
		status, meta, err := w.nomad.Jobs().ScaleStatus(w.jobID, q)

		// Any error returned can mean one of two things. If the error string
		// contains 404, this indicates the job has been purged from the
		// cluster. This is terminal and results in the watcher exiting. Other
		// errors can be seen as ephemeral and retryable.
		if err != nil && strings.Contains(err.Error(), "404") {
			w.stop()
			w.log.Info("exiting watcher due to terminal error", "error", err)
			return
		} else if err != nil {
			w.log.Error("failed to call the Nomad job scale status API", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Send the status to the channel.
		w.updateChan <- status

		maxFound = blocking.FindMaxFound(meta.LastIndex, maxFound)

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex
		w.lastChangeIndex = maxFound
	}
}

// stop helps set objects on the watcher to their stopped state setting.
func (w *Watcher) stop() {
	w.lastChangeIndex = 0
	w.IsRunning = false
}
