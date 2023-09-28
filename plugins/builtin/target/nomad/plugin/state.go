// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/blocking"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
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

var (
	// statusHandlerInitTimeout is the time limit a status handler must
	// initialize before considering the operation a failure.
	// Declared as a var instead of a const to allow overwriting it in tests.
	statusHandlerInitTimeout = 30 * time.Second
)

// jobScaleStatusHandler is an individual handler on the /v1/job/<job>/scale
// GET endpoint. It provides methods for obtaining the current scaling state of
// a job and task group.
type jobScaleStatusHandler struct {
	client *api.Client
	logger hclog.Logger

	namespace string
	jobID     string

	// lock is used to synchronize access to the status variables below.
	lock sync.RWMutex

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

func newJobScaleStatusHandler(client *api.Client, ns, jobID string, logger hclog.Logger) (*jobScaleStatusHandler, error) {
	jsh := &jobScaleStatusHandler{
		client:      client,
		initialDone: make(chan bool),
		jobID:       jobID,
		namespace:   ns,
		logger:      logger.With(configKeyJobID, jobID),
	}

	go jsh.start()

	// Wait for initial status data to be loaded.
	// Set a timeout to make sure callers are not blocked indefinitely.
	select {
	case <-jsh.initialDone:
	case <-time.After(statusHandlerInitTimeout):
		jsh.setStopState()
		return nil, fmt.Errorf("timeout while waiting for job scale status handler")
	}

	return jsh, nil
}

// status returns the cached scaling status of the passed group.
func (jsh *jobScaleStatusHandler) status(group string) (*sdk.TargetStatus, error) {
	jsh.lock.RLock()
	defer jsh.lock.RUnlock()

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
	resp := sdk.TargetStatus{
		Ready: !jsh.scaleStatus.JobStopped,
		Count: int64(status.Running),
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
		resp.Meta[sdk.TargetStatusMetaKeyLastEvent] = strconv.FormatUint(status.Events[0].Time, 10)
	}

	return &resp, nil
}

// start runs the blocking query loop that processes changes from the API and
// reflects the status internally.
func (jsh *jobScaleStatusHandler) start() {

	// Log that we are starting, useful for debugging.
	jsh.logger.Debug("starting job status handler")

	jsh.lock.Lock()
	jsh.isRunning = true
	jsh.lock.Unlock()

	q := &api.QueryOptions{
		Namespace: jsh.namespace,
		WaitTime:  5 * time.Minute,
		WaitIndex: 1,
	}

	for {
		status, meta, err := jsh.client.Jobs().ScaleStatus(jsh.jobID, q)

		// Update the handlers state.
		jsh.updateStatusState(status, err)

		if err != nil {
			// If the job is not found on the cluster, stop the handlers loop
			// process and set terminal state. It is still possible to read the
			// state from the handler until it is deleted by the GC.
			if errHelper.APIErrIs(err, http.StatusNotFound, "not found") {
				jsh.logger.Debug("job gone away", "message", err)
				jsh.setStopState()
				return
			}

			// Reset query WaitIndex to zero so we can get the job status
			// immediately in the next request instead of blocking and having
			// to wait for a timeout.
			q.WaitIndex = 0

			// If the error was anything other than the job not being found,
			// try again.
			jsh.logger.Warn("failed to read job scale status, retrying in 10 seconds", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// Read handler state into local variables so we don't starve the lock.
		jsh.lock.RLock()
		isRunning := jsh.isRunning
		scaleStatus := jsh.scaleStatus
		jsh.lock.RUnlock()

		// Stop loop if handler is not running anymore.
		if !isRunning {
			return
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		// The index could also be the same when a reconnect happens, in which
		// case the handler state needs to be updated regardless of the index.
		if scaleStatus != nil && !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Modify the wait index on the QueryOptions so the blocking query
		// is using the latest index value.
		q.WaitIndex = meta.LastIndex
	}
}

// updateStatusState takes the API responses and updates the internal state
// along with a timestamp.
func (jsh *jobScaleStatusHandler) updateStatusState(status *api.JobScaleStatusResponse, err error) {
	jsh.lock.Lock()
	defer jsh.lock.Unlock()

	// Mark the handler as initialized and notify initialDone channel.
	if !jsh.initialized {
		jsh.initialized = true

		// Close channel so we don't block waiting for readers.
		// jsh.initialized should only be set once to avoid closing this twice.
		close(jsh.initialDone)
	}

	jsh.scaleStatus = status
	jsh.scaleStatusError = err
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}

// setStopState handles updating state when the job status handler is going to
// stop.
func (jsh *jobScaleStatusHandler) setStopState() {
	jsh.lock.Lock()
	defer jsh.lock.Unlock()

	jsh.isRunning = false
	jsh.scaleStatus = nil
	jsh.scaleStatusError = nil
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}

// shouldGC returns true if the handler is not considered as active anymore.
func (jsh *jobScaleStatusHandler) shouldGC(threshold int64) bool {
	jsh.lock.RLock()
	defer jsh.lock.RUnlock()

	return !jsh.isRunning && jsh.lastUpdated < threshold
}
