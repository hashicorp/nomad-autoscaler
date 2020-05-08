package nomad

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
)

const (
	// metaKeyPrefix is the key prefix to be used when adding items to the
	// status response meta object.
	metaKeyPrefix = "nomad_autoscaler.target.nomad."

	// metaKeyJobStoppedSuffix is the key suffix used when adding a meta item
	// to the status response detailing the jobs current stopped status.
	metaKeyJobStoppedSuffix = ".stopped"
)

// jobScaleStatusHandler is an individual handler on the /v1/job/<job>/scale
// GET endpoint. It provides methods for obtaining the current scaling state of
// a job and task group.
type jobScaleStatusHandler struct {
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
	initialized bool

	// isRunning details whether the loop within start() is currently running
	// or not.
	isRunning bool

	// lastUpdated is the UnixNano UTC timestamp of the last update to the
	// state. This helps with garbage collection.
	lastUpdated int64
}

func newJobScaleStatusHandler(client *api.Client, jobID string, logger hclog.Logger) *jobScaleStatusHandler {
	return &jobScaleStatusHandler{
		client:      client,
		initialDone: make(chan bool),
		jobID:       jobID,
		logger:      logger.With(configKeyJobID, jobID),
	}
}

// status returns the cached scaling status of the passed group.
func (jsh *jobScaleStatusHandler) status(group string) (*target.Status, error) {

	// If the last status response included an error, just return this to the
	// caller.
	if jsh.scaleStatusError != nil {
		return nil, jsh.scaleStatusError
	}

	// If the scale status is nil, it means the main loop is stopped and
	// therefore the job is not found on the cluster.
	if jsh.scaleStatus == nil {
		return nil, nil
	}

	// Use a variable to sort the task group status if we find it. Using a
	// pointer allows us to perform a nil check to see if we found the task
	// group or not.
	var status *api.TaskGroupScaleStatus

	// Iterate the task groups until we find the one we are looking for.
	for name, tg := range jsh.scaleStatus.TaskGroups {
		if name == group {
			status = &tg
			break
		}
	}

	// If we did not find the task group in the status list, we can't reliably
	// inform the caller of any details. Therefore return an error.
	if status == nil {
		return nil, fmt.Errorf("task group %q not found", group)
	}

	// Hydrate the response object with the information we have collected that
	// is nil safe.
	// TODO(jrasell) Currently "Running" is not populated in a tagged release:
	//  https://github.com/hashicorp/nomad/issues/7789
	resp := target.Status{
		Ready: !jsh.scaleStatus.JobStopped,
		Count: int64(status.Healthy),
		Meta: map[string]string{
			metaKeyPrefix + jsh.jobID + metaKeyJobStoppedSuffix: strconv.FormatBool(jsh.scaleStatus.JobStopped),
		},
	}

	// Scaling events are an ordered list. If we have entries take the
	// timestamp of the most recent and add this to our meta.
	//
	// Currently any event registered will cause the cooldown period to take
	// effect. If we use the scale endpoint in the future to register events
	// such as policy parsing errors, we should filter those out.
	if len(status.Events) > 0 {
		resp.Meta[target.MetaKeyLastEvent] = strconv.FormatUint(status.Events[0].Time, 10)
	}

	return &resp, nil
}

// start runs the blocking query loop that processes changes from the API and
// reflects the status internally.
func (jsh *jobScaleStatusHandler) start() {

	// Log that we are starting, useful for debugging.
	jsh.logger.Debug("starting job status handler")
	jsh.isRunning = true

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		status, meta, err := jsh.client.Jobs().ScaleStatus(jsh.jobID, q)
		if err != nil {

			// If the job is not found on the cluster, stop the handlers loop
			// process and set terminal state. It is still possible to read the
			// state from the handler until it is deleted by the GC.
			if strings.Contains(err.Error(), "404") {
				jsh.setStopState()
				return
			}
			jsh.updateStatusState(status, err)

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

		// Mark the handler as initialized and notify initialDone channel.
		if !jsh.initialized {
			jsh.handleFirstRun()
			jsh.initialized = true
		}

		// Modify the wait index on the QueryOptions so the blocking query
		// is using the latest index value.
		q.WaitIndex = meta.LastIndex
	}
}

// handleFirstRun is a helper function which responds to channel listeners that
// the first run of the blocking query has completed and therefore data is
// available for querying.
func (jsh *jobScaleStatusHandler) handleFirstRun() { jsh.initialDone <- true }

// updateStatusState takes the API responses and updates the internal state
// along with a timestamp.
func (jsh *jobScaleStatusHandler) updateStatusState(status *api.JobScaleStatusResponse, err error) {
	jsh.scaleStatus = status
	jsh.scaleStatusError = err
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}

// setStopState handles updating state when the job status handler is going to
// stop.
func (jsh *jobScaleStatusHandler) setStopState() {
	jsh.isRunning = false
	jsh.scaleStatus = nil
	jsh.scaleStatusError = nil
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}
