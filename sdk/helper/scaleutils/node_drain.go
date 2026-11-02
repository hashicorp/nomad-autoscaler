// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/blocking"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
)

const (
	defaultNodeDrainDeadline    = 15 * time.Minute
	defaultNodeIgnoreSystemJobs = false

	nodeDrainedMetaKey   = "drained_by"
	nodeDrainedMetaValue = "nomad-autoscaler"
)

// DrainNodes iterates the provided nodeID list and performs a drain on each
// one. Each node drain is monitored and events logged until the context is
// closed or all drains reach a terminal state.
func (c *ClusterScaleUtils) DrainNodes(ctx context.Context, cfg map[string]string, nodes []NodeResourceID) error {

	drainSpec, err := drainSpec(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate node drainspec: %v", err)
	}

	// Define a WaitGroup. This allows us to trigger each node drain in a go
	// routine and then wait for them all to complete before exiting.
	var wg sync.WaitGroup
	wg.Add(len(nodes))

	// Define an error to collect errors from each drain routine and a mutex to
	// provide thread safety when calling multierror.Append.
	var (
		result     *multierror.Error
		resultLock sync.Mutex
	)

	for _, node := range nodes {

		// Assign our node to a local variable as we are launching a go routine
		// from within a for loop.
		n := node

		// Launch a routine to drain the node. Append any error returned to the
		// error.
		go func() {

			// Ensure we call done on the WaitGroup to decrement the count remaining.
			defer wg.Done()

			if err := c.drainNode(ctx, n.NomadNodeID, drainSpec); err != nil {
				resultLock.Lock()
				result = multierror.Append(result, err)
				resultLock.Unlock()
			}
			c.log.Debug("node drain complete", "node_id", n.NomadNodeID)
		}()
	}

	wg.Wait()

	return errHelper.FormattedMultiError(result)
}

// drainSpec generates the Nomad API node drain specification based on the user
// configuration. Any options which have attempted to be configured, but are
// malformed are considered a terminal error. This allows operators to correct
// any mistakes and ensures we do not assume values.
func drainSpec(cfg map[string]string) (*api.DrainSpec, error) {

	// The configuration options that define the drain spec are optional and
	// have sensible defaults.
	deadline := defaultNodeDrainDeadline
	ignoreSystemJobs := defaultNodeIgnoreSystemJobs

	// Use a multierror so we can report all errors in a single call. This
	// allows for faster resolution and a nicer UX.
	var mErr *multierror.Error

	// Attempt to read the operator defined deadline from the config.
	if drainString, ok := cfg[sdk.TargetConfigKeyDrainDeadline]; ok {
		d, err := time.ParseDuration(drainString)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			deadline = d
		}
	}

	// Attempt to read the operator defined ignore system jobs from the config.
	if ignoreSystemJobsString, ok := cfg[sdk.TargetConfigKeyIgnoreSystemJobs]; ok {
		isj, err := strconv.ParseBool(ignoreSystemJobsString)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			ignoreSystemJobs = isj
		}
	}

	// Check whether we have found errors, and return these in a nicely
	// formatted way.
	if mErr != nil {
		return nil, errHelper.FormattedMultiError(mErr)
	}

	return &api.DrainSpec{
		Deadline:         deadline,
		IgnoreSystemJobs: ignoreSystemJobs,
	}, nil
}

// drainNode triggers a drain on the supplied ID using the DrainSpec. The
// function handles monitoring the drain and reporting its terminal status to
// the caller.
func (c *ClusterScaleUtils) drainNode(ctx context.Context, nodeID string, spec *api.DrainSpec) error {

	c.log.Info("triggering drain on node", "node_id", nodeID, "deadline", spec.Deadline)

	opts := &api.DrainOptions{
		DrainSpec:    spec,
		MarkEligible: false,
		Meta: map[string]string{
			nodeDrainedMetaKey: nodeDrainedMetaValue,
		},
	}

	// Update the drain on the node.
	resp, err := c.drainer.UpdateDrainOpts(nodeID, opts, nil)
	if err != nil {
		return fmt.Errorf("failed to drain node: %w", err)
	}

	// Monitor the drain so we output the log messages. An error here indicates
	// the drain failed to complete successfully.
	if err := c.monitorNodeDrain(ctx, nodeID, resp.LastIndex, spec.IgnoreSystemJobs); err != nil {
		return fmt.Errorf("context done while monitoring node drain: %w", err)
	}

	// Drain completion is based on allocation status and does not guarantee
	// all tasks have finished. Wait until all tasks are dead before proceeding.
	if err := c.waitForAllTasksDead(ctx, nodeID, spec.IgnoreSystemJobs); err != nil {
		return fmt.Errorf("failed waiting for all tasks to be dead on node %s: %w", nodeID, err)
	}

	return nil
}

// waitForAllTasksDead blocks until all tasks across all allocations on the
// node are dead, or the context is cancelled. This ensures that any remaining work
// on the node has completed before the caller proceeds with node termination.
//
// When ignoreSystemJobs is true, the drainer deliberately leaves system job
// allocations running, so they are excluded from the check. Otherwise this
// would block until the context is cancelled.
func (c *ClusterScaleUtils) waitForAllTasksDead(ctx context.Context, nodeID string, ignoreSystemJobs bool) error {
	q := &api.QueryOptions{
		WaitIndex: 0,
	}

	// Job types never change for a given job, so cache lookups across polls.
	jobTypes := make(map[string]string)

	for {
		allocs, meta, err := c.client.Nodes().Allocations(nodeID, q.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to list allocations for node: %w", err)
		}

		if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		allDead := true
	allocCheck:
		for _, alloc := range allocs {
			if ignoreSystemJobs {
				system, err := c.isSystemJobAlloc(ctx, alloc, jobTypes)
				if err != nil {
					return err
				}
				if system {
					continue
				}
			}

			for taskName, taskState := range alloc.TaskStates {
				if taskState.State != "dead" {
					c.log.Info("waiting for task to finish before terminating node",
						"node_id", nodeID,
						"alloc_id", alloc.ID,
						"task", taskName,
						"state", taskState.State,
					)
					allDead = false
					break allocCheck
				}
			}
		}

		if allDead {
			return nil
		}

		q.WaitIndex = meta.LastIndex
	}
}

// isSystemJobAlloc reports whether the allocation belongs to a job of type
// system. This mirrors the Nomad drainer, which only skips jobs of this exact
// type when ignoring system jobs (sysbatch jobs are still drained). The job
// type is taken from the allocation when present, otherwise it is looked up
// and stored in the supplied cache.
func (c *ClusterScaleUtils) isSystemJobAlloc(ctx context.Context, alloc *api.Allocation, cache map[string]string) (bool, error) {
	if alloc.Job != nil && alloc.Job.Type != nil {
		return *alloc.Job.Type == api.JobTypeSystem, nil
	}

	key := alloc.Namespace + "/" + alloc.JobID
	if jobType, ok := cache[key]; ok {
		return jobType == api.JobTypeSystem, nil
	}

	q := &api.QueryOptions{Namespace: alloc.Namespace}
	job, _, err := c.client.Jobs().Info(alloc.JobID, q.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to read job %q for allocation %s: %w", alloc.JobID, alloc.ID, err)
	}

	jobType := ""
	if job.Type != nil {
		jobType = *job.Type
	}
	cache[key] = jobType

	return jobType == api.JobTypeSystem, nil
}

// monitorNodeDrain follows the drain of a node, logging the messages we
// receive to their appropriate level.
func (c *ClusterScaleUtils) monitorNodeDrain(ctx context.Context, nodeID string, index uint64, ignoreSys bool) error {
	for msg := range c.drainer.MonitorDrain(ctx, nodeID, index, ignoreSys) {

		switch msg.Level {
		case api.MonitorMsgLevelInfo:
			c.log.Info("received node drain message", "node_id", nodeID, "msg", msg.Message)
		case api.MonitorMsgLevelWarn:
			c.log.Warn("received node drain message", "node_id", nodeID, "msg", msg.Message)
		case api.MonitorMsgLevelError:
			return fmt.Errorf("received error while draining node: %s", msg.Message)
		default:
			c.log.Debug("received node drain message", "node_id", nodeID, "msg", msg.Message)
		}
	}
	return ctx.Err()
}

// RunPostScaleInTasksOnFailure is a utility that runs tasks on nodes which
// were unable to be successfully terminated. The current tasks are:
//
//   - modify node eligibility to true
func (c *ClusterScaleUtils) RunPostScaleInTasksOnFailure(nodes []NodeResourceID) error {

	var mErr *multierror.Error

	for _, node := range nodes {
		resp, err := c.client.Nodes().ToggleEligibility(node.NomadNodeID, true, nil)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("failed to toggle eligibility on node %s: %v",
				node.NomadNodeID, err))
			continue
		}
		c.log.Info("successfully toggled node eligibility",
			"node_id", node.NomadNodeID, "evals", resp.EvalIDs)
	}

	return errHelper.FormattedMultiError(mErr)
}
