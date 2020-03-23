package status

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad/api"
)

type Watcher struct {
	log             hclog.Logger
	jobID           string
	nomad           *api.Client
	lastChangeIndex uint64
	updateChan      chan *api.JobScaleStatusResponse
	InitialLoad     chan bool
	initialRun      bool
}

// NewWatcher creates a new scale status watcher, using blocking queries to
// monitor changes from Nomad.
func NewWatcher(log hclog.Logger, jobID string, nomad *api.Client, updateChan chan *api.JobScaleStatusResponse) *Watcher {
	return &Watcher{
		InitialLoad: make(chan bool, 1),
		initialRun:  true,
		jobID:       jobID,
		log:         log.Named("status-watcher-" + jobID),
		nomad:       nomad,
		updateChan:  updateChan,
	}
}

func (w *Watcher) Start() {
	var maxFound uint64

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		status, meta, err := w.nomad.Jobs().ScaleStatus(w.jobID, q)
		if err != nil {
			w.log.Error("failed to call the Nomad job scale status API", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChange(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Send the scale status to the channel.
		w.updateChan <- status

		maxFound = blocking.FindMaxFound(meta.LastIndex, maxFound)

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex
		w.lastChangeIndex = maxFound

		// If this is our initial run, inform any subscribers we are finished.
		if w.initialRun {
			w.InitialLoad <- true
			close(w.InitialLoad)
			w.initialRun = false
		}
	}
}
