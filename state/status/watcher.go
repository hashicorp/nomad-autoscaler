package status

import (
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
	updateFunc      updateFunc

	// InitialLoad helps callers understand when the initial run of the Start()
	// loop has finished. This results in our status state being populated
	// which is required when first processing a scaling policy.
	InitialLoad chan bool
	initialRun  bool
}

// updateFunc is the function that is used to reflect an update in a job
// scaling status Nomad information locally. This differs from the policy
// watcher (which uses a channel to process updates) in order to solve race a
// condition problem when using channels. This condition is that when
// processing a policy for the first time, we need to ensure we have local
// status state to check the JobStopped field. Using channels for this has no
// timing guarantees unless adding complex logic. It is simpler and easier to
// just use a blocking function to ensure the ordering.
type updateFunc func(update *api.JobScaleStatusResponse)

// NewWatcher creates a new scale status watcher, using blocking queries to
// monitor changes from Nomad.
func NewWatcher(log hclog.Logger, jobID string, nomad *api.Client, updateFunc updateFunc) *Watcher {
	return &Watcher{
		InitialLoad: make(chan bool, 1),
		initialRun:  true,
		jobID:       jobID,
		log:         log.Named("status_watcher_" + jobID),
		nomad:       nomad,
		updateFunc:  updateFunc,
	}
}

// Start starts the query loop that performs a blocking query on the scaling
// status endpoint of a specific job.
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

		// Run the update function so the new status information is persisted
		// in the internal store.
		w.updateFunc(status)

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
