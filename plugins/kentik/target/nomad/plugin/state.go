// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/pkg/node"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/blocking"
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

type UnknownJobAllocs map[string]int
type IneligibleUnknownAllocs map[string]int

// jobScaleStatusHandler is an individual handler on the /v1/job/<job>/scale
// GET endpoint. It provides methods for obtaining the current scaling state of
// a job and task group.
type jobScaleStatusHandler struct {
	client *api.Client
	logger hclog.Logger
	nodes  node.Status

	namespace          string
	jobID              string
	checkUnknownAllocs bool

	// lock is used to synchronize access to the status variables below.
	lock sync.RWMutex

	// scaleStatus is the internal reflection of the response objects from the
	// job scale status API.
	scaleStatus             *api.JobScaleStatusResponse
	unknownAllocs           UnknownJobAllocs
	ineligibleUnknownAllocs IneligibleUnknownAllocs
	scaleStatusError        error
	allocStatusError        error

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

func newJobScaleStatusHandler(client *api.Client, ns, jobID string, checkUnknownAllocs bool, logger hclog.Logger, nodes node.Status) (*jobScaleStatusHandler, error) {
	jsh := &jobScaleStatusHandler{
		client:                  client,
		initialDone:             make(chan bool),
		jobID:                   jobID,
		namespace:               ns,
		checkUnknownAllocs:      checkUnknownAllocs,
		logger:                  logger.With(configKeyJobID, jobID),
		nodes:                   nodes,
		ineligibleUnknownAllocs: IneligibleUnknownAllocs{},
		unknownAllocs:           UnknownJobAllocs{},
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

	if jsh.checkUnknownAllocs {
		if jsh.allocStatusError != nil {
			jsh.logger.Info("returning alloc status error")
			return nil, jsh.allocStatusError
		}
		if count, exists := jsh.unknownAllocs[group]; exists {
			jsh.logger.Info("returning status error because there are unknown allocs")
			return nil, fmt.Errorf("group %s contains %d unknown allocs", group, count)
		}
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

	count := int64(status.Running)

	if ineligible, exists := jsh.ineligibleUnknownAllocs[group]; exists {
		jsh.logger.Info(
			"adjusting target count since group has unknown allocs running on ineligible nodes. APM plugin should force a downscale",
			"group", group,
			"ineligible", ineligible,
			"count", count,
		)
		count += int64(ineligible)
	}
	if status.Desired > status.Running {
		jsh.logger.Warn("desired count is bigger than running count, job is probably degraded", group, group, "desired", status.Desired, "running", status.Running)
	}

	// Hydrate the response object with the information we have collected that
	// is nil safe.
	resp := sdk.TargetStatus{
		Ready: !jsh.scaleStatus.JobStopped,
		Count: count,
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

		var (
			allocs    []*api.AllocationListStub
			allocsErr error
		)

		if err == nil && jsh.checkUnknownAllocs {
			jsh.logger.Debug("running unknown allocs evaluation", "job", jsh.jobID)
			allocs, _, allocsErr = jsh.client.Jobs().Allocations(jsh.jobID, true, &api.QueryOptions{Namespace: jsh.namespace})
			if allocsErr != nil {
				jsh.logger.Debug("failed to read job allocation status, the error will be ignored", "error", err)
			}
		}

		// Update the handlers state.
		jsh.updateStatusState(status, allocs, err, allocsErr)

		if err != nil {
			jsh.logger.Debug("failed to read job scale status, retrying", "error", err)

			// If the job is not found on the cluster, stop the handlers loop
			// process and set terminal state. It is still possible to read the
			// state from the handler until it is deleted by the GC.
			if strings.Contains(err.Error(), "404") {
				jsh.setStopState()
				return
			}

			// Reset query WaitIndex to zero so we can get the job status
			// immediately in the next request instead of blocking and having
			// to wait for a timeout.
			q.WaitIndex = 0

			// If the error was anything other than the job not being found,
			// try again.
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
func (jsh *jobScaleStatusHandler) updateStatusState(status *api.JobScaleStatusResponse, allocs []*api.AllocationListStub, err, allocsErr error) {
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
	jsh.allocStatusError = allocsErr
	jsh.unknownAllocs, jsh.ineligibleUnknownAllocs = jsh.toUnknownAllocs(allocs)
	jsh.lastUpdated = time.Now().UTC().UnixNano()
}

func (jsh *jobScaleStatusHandler) toUnknownAllocs(allocs []*api.AllocationListStub) (UnknownJobAllocs, IneligibleUnknownAllocs) {
	allocStatus := UnknownJobAllocs{}
	ineligibleStatus := IneligibleUnknownAllocs{}

	for _, alloc := range allocs {
		if alloc.DesiredStatus != "run" {
			continue
		}
		if alloc.ClientStatus == "unknown" {
			if jsh.runningOnEligibleNode(alloc.NodeID) {
				jsh.logger.Info("group contains unknown allocs", "group", alloc.TaskGroup)
				allocStatus[alloc.TaskGroup] = allocStatus[alloc.TaskGroup] + 1
			} else {
				jsh.logger.Info("group contains unknown allocs but node is ineligible", "group", alloc.TaskGroup, "node", alloc.NodeID)
				ineligibleStatus[alloc.TaskGroup] = ineligibleStatus[alloc.TaskGroup] + 1
			}
		}
	}

	return allocStatus, ineligibleStatus
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

func (jsh *jobScaleStatusHandler) runningOnEligibleNode(id string) bool {
	return !jsh.nodes.IsIneligible(id)
}
