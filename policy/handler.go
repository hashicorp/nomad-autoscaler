// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	cooldownIgnoreTime = 3 * time.Minute
)

// Handler monitors a policy for changes and controls when them are sent for
// evaluation.
type Handler struct {
	log hclog.Logger

	// mutators is a list of mutations to apply to policies.
	mutators []Mutator

	// ticker controls the frequency the policy is sent for evaluation.
	ticker *time.Ticker

	// ch is used to listen for policy updates.
	updatesCh <-chan *sdk.ScalingPolicy

	policyLock sync.RWMutex
	policy     *sdk.ScalingPolicy

	statusGetter targetpkg.TargetStatusGetter

	// cooldownCh is used to notify the handler that it should enter a cooldown
	// period.
	cooldownCh <-chan time.Duration
	errChn     chan<- error
}

//	StartNewHandler starts the handler for the given policy
//
// This function blocks until the context provided is canceled.
type HandlerConfig struct {
	CooldownChan chan time.Duration
	UpdatesChan  chan *sdk.ScalingPolicy
	ErrChan      chan<- error
	Policy       *sdk.ScalingPolicy
	Log          hclog.Logger
	TargetGetter targetpkg.TargetStatusGetter
	EvalsChannel chan<- *sdk.ScalingEvaluation
}

func RunNewHandler(ctx context.Context, config HandlerConfig) {

	h := &Handler{
		log: config.Log.Named("policy_handler").With("policy_id", config.Policy.ID),
		mutators: []Mutator{
			NomadAPMMutator{},
		},
		cooldownCh:   config.CooldownChan,
		statusGetter: config.TargetGetter,
		updatesCh:    config.UpdatesChan,
		policy:       config.Policy,
		errChn:       config.ErrChan,
	}

	h.ticker = time.NewTicker(h.policy.EvaluationInterval)
	defer h.ticker.Stop()

	h.log.Trace("starting policy handler")
	for {
		select {
		case <-ctx.Done():
			h.log.Error("stopping policy handler due to context done")
			return

		case updatedPolicy := <-h.updatesCh:
			err := updatedPolicy.Validate()
			if err != nil {
				h.errChn <- fmt.Errorf("invalid policy: %v", err)
			}

			h.applyMutators(updatedPolicy)
			h.updateHandler(updatedPolicy)

		case <-h.ticker.C:
			eval, err := h.handleTick(ctx, h.policy)
			if err != nil {
				h.errChn <- fmt.Errorf("handler: unable to process policy %v", err)
			}

			if eval != nil {
				config.EvalsChannel <- eval
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

func (h *Handler) handleTick(ctx context.Context, policy *sdk.ScalingPolicy) (*sdk.ScalingEvaluation, error) {
	h.log.Trace("handling tick")

	// Timestamp the invocation of this evaluation run. This can be
	// used when checking cooldown or emitting metrics to ensure some
	// consistency.
	curTime := time.Now().UTC().UnixNano()

	status, err := h.statusGetter.Status(policy.Target.Config)
	if err != nil {
		h.log.Warn("failed to get target status", "error", err)
		return nil, err
	}

	// A nil status indicates the target doesn't exist, so we don't need to
	// monitor the policy anymore.
	if status == nil {
		h.log.Trace("target doesn't exist anymore", "target", policy.Target.Config)
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
func (h *Handler) updateHandler(updatedPolicy *sdk.ScalingPolicy) {
	h.log.Trace("updating handler", "policy_id", updatedPolicy.ID)

	// Update ticker if the policy's evaluation interval has changed.
	if h.policy.EvaluationInterval != updatedPolicy.EvaluationInterval {
		h.ticker.Stop()

		// Add a small random delay between 0 and 300ms to spread the first
		// evaluation of policies that are loaded at the same time.
		splayNs := rand.Intn(30) * 100 * 1000 * 1000
		time.Sleep(time.Duration(splayNs))

		h.ticker = time.NewTicker(updatedPolicy.EvaluationInterval)
	}

	h.policyLock.Lock()
	defer h.policyLock.Unlock()

	h.policy = updatedPolicy
}

// enforceCooldown blocks until the cooldown period has been reached, or the
// handler has been instructed to exit. The boolean return details whether or
// not the cooldown period passed without being interrupted.
func (h *Handler) enforceCooldown(ctx context.Context, t time.Duration) bool {

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
	case <-ctx.Done():
		return false
	case <-timer.C:
	}

	return true
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
