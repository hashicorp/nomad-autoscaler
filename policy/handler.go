// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
)

const (
	cooldownIgnoreTime = 1 * time.Second
)

// Handler monitors a policy for changes and controls when them are sent for
// evaluation.
type Handler struct {
	log hclog.Logger

	// policyID is the ID of the policy the handler is responsible for.
	policyID PolicyID

	// pluginManager is used to retrieve an instance of the target plugin used
	// by the policy.
	pluginManager *manager.PluginManager

	// policySource is used to monitor for changes to the policy the handler
	// is responsible for.
	policySource Source

	// mutators is a list of mutations to apply to policies.
	mutators []Mutator

	// ticker controls the frequency the policy is sent for evaluation.
	ticker *time.Ticker

	// cooldownCh is used to notify the handler that it should enter a cooldown
	// period.
	cooldownCh chan time.Duration

	// running is used to help keep track if the handler is active or not.
	running     bool
	runningLock sync.RWMutex

	// ch is used to listen for policy updates.
	ch chan sdk.ScalingPolicy

	// errCh is used to listen for errors from the policy source.
	errCh chan error

	// doneCh is used to signal the handler to stop.
	doneCh chan struct{}

	// reloadCh is used to communicate to the MonitorPolicy routine that it
	// should perform a reload.
	reloadCh chan struct{}
}

// NewHandler returns a new handler for a policy.
func NewHandler(ID PolicyID, log hclog.Logger, pm *manager.PluginManager, ps Source) *Handler {
	return &Handler{
		policyID:      ID,
		log:           log.Named("policy_handler").With("policy_id", ID),
		pluginManager: pm,
		policySource:  ps,
		mutators: []Mutator{
			NomadAPMMutator{},
		},
		ch:         make(chan sdk.ScalingPolicy),
		errCh:      make(chan error),
		doneCh:     make(chan struct{}),
		cooldownCh: make(chan time.Duration, 1),
		reloadCh:   make(chan struct{}),
	}
}

// Run starts the handler and periodically sends the policy for evaluation.
//
// This function blocks until the context provided is canceled or the handler
// is stopped with the Stop() method.
func (h *Handler) Run(ctx context.Context, evalCh chan<- *sdk.ScalingEvaluation) {
	h.log.Trace("starting policy handler")

	defer h.Stop()

	// Mark the handler as running.
	h.runningLock.Lock()
	h.running = true
	h.runningLock.Unlock()

	// Store a local copy of the policy so we can compare it for changes.
	var currentPolicy *sdk.ScalingPolicy

	// Start with a long ticker until we receive the right interval.
	// TODO(luiz): make this a config param
	policyReadTimeout := 3 * time.Minute
	h.ticker = time.NewTicker(policyReadTimeout)

	// Create separate context so we can stop the monitoring Go routine if
	// doneCh is closed, but ctx is still valid.
	monitorCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start monitoring the policy for changes.
	req := MonitorPolicyReq{ID: h.policyID, ErrCh: h.errCh, ReloadCh: h.reloadCh, ResultCh: h.ch}
	go h.policySource.MonitorPolicy(monitorCtx, req)

	for {
		select {
		case <-ctx.Done():
			h.log.Trace("stopping policy handler due to context done")
			return
		case <-h.doneCh:
			h.log.Trace("stopping policy handler due to done channel")
			return

		case err := <-h.errCh:
			// In case of error, log the error message and loop around.
			// Handlers never stop running unless ctx.Done() or doneCh is
			// closed.
			if err == nil {
				continue
			}

			// multierror.Error objects are logged differently to allow for a
			// more structured output.
			merr, ok := err.(*multierror.Error)
			if ok && len(merr.Errors) > 1 {
				// Transform Errors into a slice of strings to avoid logging
				// empty objects when using JSON format.
				errors := make([]string, len(merr.Errors))
				for i, e := range merr.Errors {
					errors[i] = e.Error()
				}
				h.log.Error(errors[0], "errors", errors[1:])
			} else {
				if errHelper.APIErrIs(err, http.StatusNotFound, "not found") {
					// the policy is gone, so this handler should stop,
					// but the monitor manager will handle that.
					// here we just log for potential future debugging.
					h.log.Debug("policy gone away", "message", err)
				} else {
					h.log.Error("encountered an error monitoring policy", "error", err)
				}
			}
			continue

		case p := <-h.ch:
			h.applyMutators(&p)
			h.updateHandler(currentPolicy, &p)
			currentPolicy = &p

		case <-h.ticker.C:
			eval, err := h.handleTick(ctx, currentPolicy)
			if err != nil {
				if err == context.Canceled {
					// Context was canceled, return to stop the handler.
					return
				}
				h.log.Error(err.Error())
				continue
			}

			if eval != nil {
				evalCh <- eval
			}

		case ts := <-h.cooldownCh:
			// Enforce the cooldown which will block until complete.
			if !h.enforceCooldown(ctx, ts) {
				// Context was canceled, return to stop the handler.
				return
			}
		}
	}
}

// Stop stops the handler and the monitoring Go routine.
func (h *Handler) Stop() {
	h.runningLock.Lock()
	defer h.runningLock.Unlock()

	if h.running {
		h.log.Trace("stopping handler")
		h.ticker.Stop()
		close(h.doneCh)
	}

	h.running = false
}

func (h *Handler) handleTick(ctx context.Context, policy *sdk.ScalingPolicy) (*sdk.ScalingEvaluation, error) {
	h.log.Trace("tick")

	if policy == nil {
		// Initial ticker ticked without a policy being set, assume we are not able
		// to retrieve the policy and exit.
		return nil, errors.New("timeout: failed to read policy in time")
	}

	// Validate policy on ticker so any validation errors are resurfaced
	// periodically.
	err := policy.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid policy: %v", err)
	}

	// Timestamp the invocation of this evaluation run. This can be
	// used when checking cooldown or emitting metrics to ensure some
	// consistency.
	curTime := time.Now().UTC().UnixNano()

	// Exit early if the policy is not enabled.
	if !policy.Enabled {
		h.log.Debug("policy is not enabled")
		return nil, nil
	}

	target, err := h.pluginManager.GetTarget(policy.Target)
	if err != nil {
		h.log.Warn("failed to get target", "error", err)
		return nil, err
	}

	status, err := target.Status(policy.Target.Config)
	if err != nil {
		h.log.Warn("failed to get target status", "error", err)
		return nil, err
	}

	// A nil status indicates the target doesn't exist, so we don't need to
	// monitor the policy anymore.
	if status == nil {
		h.log.Trace("target doesn't exist anymore", "target", policy.Target.Config)
		h.Stop()
		return nil, nil
	}

	// Exit early if the target is not ready yet.
	if !status.Ready {
		h.log.Trace("target is not ready")
		return nil, nil
	}

	// Send policy for evaluation.
	h.log.Trace("sending policy for evaluation")

	eval := sdk.NewScalingEvaluation(policy)
	// If the evaluation is nil there is nothing to be done this time
	// around.
	if eval == nil {
		return nil, nil
	}

	// If the target status includes a last event meta key, check for cooldown
	// due to out-of-band events. This is also useful if the Autoscaler has
	// been re-deployed.
	ts, ok := status.Meta[sdk.TargetStatusMetaKeyLastEvent]
	if !ok {
		return eval, nil
	}

	// Convert the last event string. If an error occurs, just log and
	// continue with the evaluation. A malformed timestamp shouldn't mean
	// we skip scaling.
	lastTS, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		h.log.Error("failed to parse last event timestamp as int64", "error", err)
		return eval, nil
	}

	// Calculate the remaining time period left on the cooldown. If this is
	// cooldownIgnoreTime or below, we do not need to enter cooldown. Reasoning
	// on ignoring small variations can be seen within GH-138.
	cdPeriod := h.calculateRemainingCooldown(policy.Cooldown, curTime, lastTS)
	if cdPeriod <= cooldownIgnoreTime {
		return eval, nil
	}

	// Enforce the cooldown which will block until complete. A false response
	// means we did not reach the end of cooldown due to a request to shutdown.
	if !h.enforceCooldown(ctx, cdPeriod) {
		return nil, context.Canceled
	}

	// If we reach this point, we have entered and exited cooldown. Our data is
	// stale, therefore return so that we do not send the eval this time and
	// wait for the next tick.
	return nil, nil
}

// updateHandler updates the handler's internal state based on the changes in
// the policy being monitored.
func (h *Handler) updateHandler(current, next *sdk.ScalingPolicy) {
	if current == nil {
		h.log.Trace("received policy")
	} else {
		h.log.Trace("received policy change")
		h.log.Trace(cmp.Diff(current, next))
	}

	// Update ticker if it's the first time we receive the policy or if the
	// policy's evaluation interval has changed.
	if current == nil || current.EvaluationInterval != next.EvaluationInterval {
		h.ticker.Stop()

		// Add a small random delay between 0 and 300ms to spread the first
		// evaluation of policies that are loaded at the same time.
		splayNs := rand.Intn(30) * 100 * 1000 * 1000
		time.Sleep(time.Duration(splayNs))

		h.ticker = time.NewTicker(next.EvaluationInterval)
	}
}

// enforceCooldown blocks until the cooldown period has been reached, or the
// handler has been instructed to exit. The boolean return details whether or
// not the cooldown period passed without being interrupted.
func (h *Handler) enforceCooldown(ctx context.Context, t time.Duration) (complete bool) {

	// Log that cooldown is being enforced. This is very useful as cooldown
	// blocks the ticker making this the only indication of cooldown to
	// operators.
	h.log.Debug("scaling policy has been placed into cooldown", "cooldown", t)

	// Using a timer directly is mentioned to be more efficient than
	// time.After() as long as we ensure to call Stop(). So setup a timer for
	// use and defer the stop.
	timer := time.NewTimer(t)
	defer timer.Stop()

	// Cooldown should not mean we miss other handler control signals. So wait
	// on all the channels desired here.
	select {
	case <-timer.C:
		complete = true
		return
	case <-ctx.Done():
		return
	case <-h.doneCh:
		return
	}
}

// calculateRemainingCooldown calculates the remaining cooldown based on the
// time since the last event. The remaining period can be negative, indicating
// no cooldown period is required.
func (h *Handler) calculateRemainingCooldown(cd time.Duration, ts, lastEvent int64) time.Duration {
	return cd - time.Duration(ts-lastEvent)
}

// applyMutators applies the mutators registered with the handler in order and
// log any modification that was performed.
func (h *Handler) applyMutators(p *sdk.ScalingPolicy) {
	for _, m := range h.mutators {
		for _, mutation := range m.MutatePolicy(p) {
			h.log.Info("policy modified", "modification", mutation)
		}
	}
}
