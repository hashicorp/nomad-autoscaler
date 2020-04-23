package nomad

import (
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
)

type jobStateHandler struct {
	client *api.Client
	jobID  string
	logger hclog.Logger

	// scaleStatus is the internal reflection of the response objects from the
	// job scale status API.
	scaleStatus      *api.JobScaleStatusResponse
	scaleStatusError error

	// initialDone helps synchronise the caller waiting for the state to be
	// populated after starting the API query loop.
	initialDone chan bool

	// isRunning details whether the loop within start() is currently running
	// or not.
	isRunning bool

	// lastUpdated is the UnixNano UTC timestamp of the last update to the
	// state. This helps with garbage collection.
	lastUpdated int64
}

func newJobStateHandler(client *api.Client, jobID string, logger hclog.Logger) *jobStateHandler {
	return &jobStateHandler{
		client:      client,
		initialDone: make(chan bool),
		jobID:       jobID,
		logger:      logger.With(configKeyJobID, jobID),
	}
}

func (jsh *jobStateHandler) status(group string) (*target.Status, error) {

	// If the last status response included an error, just return this to the
	// caller.
	if jsh.scaleStatusError != nil {
		return nil, jsh.scaleStatusError
	}

	var count int
	for name, tg := range jsh.scaleStatus.TaskGroups {
		if name == group {
			// Currently "Running" is not populated:
			//  https://github.com/hashicorp/nomad/issues/7789
			count = tg.Healthy
			break
		}
	}

	return &target.Status{
		Ready: !jsh.scaleStatus.JobStopped,
		Count: int64(count),
	}, nil
}

func (jsh *jobStateHandler) start() {

	jsh.isRunning = true

	defer jsh.handleFirstRun()

	q := &api.QueryOptions{WaitTime: 1 * time.Minute, WaitIndex: 1}

	for {
		status, meta, err := jsh.client.Jobs().ScaleStatus(jsh.jobID, q)
		if err != nil {
			jsh.updateStatusState(status, err)

			// If the job is not found on the cluster, stop the handlers loop
			// process and set terminal state. It is still possible to read the
			// state from the handler until it is deleted by the GC.
			if strings.Contains(err.Error(), "404") {
				jsh.setStopState()
				return
			}

			// If the error was anything other than the job not being found,
			// try again.
			time.Sleep(10 * time.Second)
			continue
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Update the handlers state.
		jsh.updateStatusState(status, nil)

		// Modify the wait index on the QueryOptions so the blocking query
		// is using the latest index value.
		q.WaitIndex = meta.LastIndex
	}
}

// handleFirstRun is a helper function which responds to channel listeners that
// the first run of the blocking query has completed and therefore data is
// available for querying.
func (jsh *jobStateHandler) handleFirstRun() { jsh.initialDone <- true }

// updateStatusState takes the API responses and updates the internal state
// along with a timestamp.
func (jsh *jobStateHandler) updateStatusState(status *api.JobScaleStatusResponse, err error) {
	jsh.scaleStatus = status
	jsh.scaleStatusError = err
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}

// setStopState handles updating state when the job status handler is going to
// stop.
func (jsh *jobStateHandler) setStopState() {
	jsh.isRunning = false
	jsh.scaleStatus = nil
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}
