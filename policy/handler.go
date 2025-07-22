// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	cooldownIgnoreTime = 3 * time.Minute
)

// errTargetNotReady is used by a check handler to indicate the policy target
// is not ready.
var errTargetNotReady = errors.New("target not ready")

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
	targetScaler targetpkg.TargetScaler

	// cooldownCh is used to notify the handler that it should enter a cooldown
	// period.
	cooldownCh chan time.Duration
	errChn     chan<- error

	checkHandlers []*checkHandler
}

type HandlerConfig struct {
	CooldownChan chan time.Duration
	UpdatesChan  chan *sdk.ScalingPolicy
	ErrChan      chan<- error
	Policy       *sdk.ScalingPolicy
	Log          hclog.Logger
	TargetGetter targetpkg.TargetStatusGetter
	EvalsChannel chan<- *sdk.ScalingEvaluation
}

//	RunNewHandler starts the handler for the given policy
//
// This function blocks until the context provided is canceled.
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

// runTargetStatus wraps the target.Status call to provide operational
// functionality.
func (h *Handler) runTargetStatus() (*sdk.TargetStatus, error) {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.policy.Target.Name}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), labels)

	return h.statusGetter.Status(h.policy.Target.Config)
}

func (h *Handler) handlePolicy(ctx context.Context) error {

	// Record the start time of the eval portion of this function. The labels
	// are also used across multiple metrics, so define them.
	evalStartTime := time.Now()
	labels := []metrics.Label{
		{Name: "policy_id", Value: h.policy.ID},
		{Name: "target_name", Value: h.policy.Target.Name},
	}

	h.log.Debug("received policy for evaluation")

	currentStatus, err := h.runTargetStatus()
	if err != nil {
		return fmt.Errorf("failed to get target status: %v", err)
	}

	if !currentStatus.Ready {
		return errTargetNotReady
	}

	// First make sure the target is within the policy limits.
	// Return early after scaling since we already modified the target.
	if currentStatus.Count < h.policy.Min {
		reason := fmt.Sprintf("scaling up because current count %d is lower than policy min value of %d",
			currentStatus.Count, h.policy.Min)

		action := sdk.ScalingAction{
			Count:     h.policy.Min,
			Reason:    reason,
			Direction: sdk.ScaleDirectionUp,
		}
		return h.scaleTarget(action, currentStatus)
	}

	if currentStatus.Count > h.policy.Max {
		reason := fmt.Sprintf("scaling down because current count %d is greater than policy max value of %d",
			currentStatus.Count, h.policy.Max)

		action := sdk.ScalingAction{
			Count:     h.policy.Max,
			Reason:    reason,
			Direction: sdk.ScaleDirectionDown,
		}
		return h.scaleTarget(action, currentStatus)
	}

	// Prepare handlers.
	handlersCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Store check results by group so we can compare their results together.
	checkGroups := make(map[string][]checkResult)

	// Start check handlers.
	for _, ch := range h.checkHandlers {
		//checkHandler := newCheckHandler(logger, eval.Policy, checkEval, w.pluginManager)

		// Wrap target status call in a goroutine so we can listen for ctx as well.
		var action *sdk.ScalingAction
		var err error
		doneCh := make(chan interface{})

		go func() {
			defer close(doneCh)
			action, err = ch.runChecks(handlersCtx, currentStatus)
		}()

		select {
		case <-ctx.Done():
			h.log.Info("stopping worker")
			return nil
		case <-doneCh:
		}

		if err != nil {
			h.log.Warn("failed to run check",
				"check", ch.check.Name,
				"on_error", ch.check.OnError,
				"on_check_error", ch.policy.OnCheckError,
				"error", err)

			// Define how to handle error.
			// Use check behaviour if set or fail iff the policy is set to fail.
			switch ch.check.OnError {
			case sdk.ScalingPolicyOnErrorIgnore:
				continue
			case sdk.ScalingPolicyOnErrorFail:
				return err
			default:
				if ch.policy.OnCheckError == sdk.ScalingPolicyOnErrorFail {
					return err
				}
			}
			continue
		}

		group := ch.check.Group
		checkGroups[group] = append(checkGroups[group], checkResult{
			action:  action,
			handler: ch,
		})
	}

	// winner is the final check that will be executed after the check groups
	// are processed.
	var winner checkResult

	for group, results := range checkGroups {
		// Decide which action wins in the group. The decision processes still
		// picks the safest choice, but it handles `none` actions a little
		// differently.
		//
		// Since grouped checks have corelated metrics, it's expected that most
		// checks will result in `none` actions as the data will be somewhere
		// else. So we ignore none actions unless _all_ checks in the group
		// vote for `none` to avoid accidentally scaling down when comparing
		// with other groups.
		var groupWinner checkResult

		noneCount := 0
		for _, r := range results {
			if r.action == nil {
				continue
			}

			if group != "" && r.action.Direction == sdk.ScaleDirectionNone {
				noneCount += 1
				continue
			}
			groupWinner = groupWinner.preempt(r)
		}

		// If all checks result in `none`, pick any one of them so when we
		// don't scale down accidentally when comparing it with other groups.
		if noneCount > 0 && noneCount == len(results) {
			groupWinner = results[0]
		}

		if groupWinner.handler == nil {
			h.log.Trace(fmt.Sprintf("no winner in group %s", group))
			continue
		}

		h.log.Debug(
			fmt.Sprintf("check %s selected in group %s", groupWinner.handler.check.Name, group),
			"direction", groupWinner.action.Direction, "count", groupWinner.action.Count)

		winner = winner.preempt(groupWinner)
	}

	// At this point the checks have finished. Therefore emit of metric data
	// tracking how long it takes to run all the checks within a policy.
	metrics.MeasureSinceWithLabels([]string{"scale", "evaluate_ms"}, evalStartTime, labels)

	if winner.handler == nil || winner.action == nil || winner.action.Direction == sdk.ScaleDirectionNone {
		h.log.Debug("no checks need to be executed")
		return nil
	}

	h.log.Debug(fmt.Sprintf("check %s selected", winner.handler.check.Name),
		"direction", winner.action.Direction, "count", winner.action.Count)

	// Measure how long it takes to invoke the scaling actions. This helps
	// understand the time taken to interact with the remote target and action
	// the scaling action.
	defer metrics.MeasureSinceWithLabels([]string{"scale", "invoke_ms"}, time.Now(), labels)

	// Last check for early exit before scaling the target, which we consider
	// a non-preemptable action since we cannot be sure that a scaling action can
	// be cancelled halfway through or undone.
	select {
	case <-ctx.Done():
		h.log.Info("stopping worker")
		return nil
	default:
	}

	err = h.scaleTarget(*winner.action, currentStatus)
	if err != nil {
		return err
	}

	h.log.Debug("policy evaluation complete")
	return nil
}

// scaleTarget performs all the necessary checks and actions necessary to scale
// a target.
func (h *Handler) scaleTarget(action sdk.ScalingAction, currentStatus *sdk.TargetStatus) error {

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := h.policy.Target.Config["dry-run"]; ok && val == "true" {
		h.log.Info("scaling dry-run is enabled, using no-op task group count")
		action.SetDryRun()
	}

	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		h.log.Debug("registering scaling event",
			"count", currentStatus.Count, "reason", action.Reason, "meta", action.Meta)
	} else {
		h.log.Info("scaling target",
			"from", currentStatus.Count, "to", action.Count,
			"reason", action.Reason, "meta", action.Meta)
	}

	metricLabels := []metrics.Label{
		{Name: "policy_id", Value: h.policy.ID},
		{Name: "target_name", Value: h.policy.Target.Name},
	}

	err := h.runTargetScale(action)
	if err != nil {
		if _, ok := err.(*sdk.TargetScalingNoOpError); ok {
			h.log.Info("scaling action skipped", "reason", err)
			return nil
		}

		metrics.IncrCounterWithLabels([]string{"scale", "invoke", "error_count"}, 1, metricLabels)
		return fmt.Errorf("failed to scale target: %w", err)
	}

	h.log.Debug("successfully submitted scaling action to target",
		"desired_count", action.Count)
	metrics.IncrCounterWithLabels([]string{"scale", "invoke", "success_count"}, 1, metricLabels)

	// Enforce the cooldown after a successful scaling event.
	//h.cooldownEnforcer.EnforceCooldown(policy.ID, calculateCooldown(policy, action))
	h.cooldownCh <- calculateCooldown(h.policy, action)
	return nil
}

func calculateCooldown(p *sdk.ScalingPolicy, a sdk.ScalingAction) time.Duration {
	if a.Direction == sdk.ScaleDirectionUp {
		return p.CooldownOnScaleUp
	}

	return p.Cooldown
}

// runTargetScale wraps the target.Scale call to provide operational
// functionality.
func (h *Handler) runTargetScale(action sdk.ScalingAction) error {
	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.policy.Target.Name}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), labels)

	return h.targetScaler.Scale(action, h.policy.Target.Config)
}
