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
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// handlerState is the representation of the current occupation of the handler,
// it works as a state machine with the following rules:
//
//     ┌─────────────────────────────────────────────────────────────────────────────┐
//     │                                                                             │
//     │                                                                             │
// ┌───▼────┐ new action   ┌────────────┐ slot  ┌───────────┐ success ┌────────────┐ │
// │ Idle   ├──────────────► OnWaiting  ┼──────►│ Scaling   ├─────────►OnCooldown  ┼─┘
// └───▲────┘              └─────┬──────┘       └─────┬─────┘         └────────────┘
//     │                         │                    │
//     │         timeout error   │  scaling error     │
//     └─────────────────────────┴────────────────────┘
//

type handlerState int

const (
	StateScaling handlerState = iota
	StateWaitingTurn
	StateCooldown
	StateIdle
)

var (
	// errTargetNotReady is used by a check handler to indicate the policy target
	// is not ready.
	errTargetNotFound = errors.New("target not found")
	errNoMetrics      = errors.New("no metrics available")
)

type dependencyGetter interface {
	GetAPMLooker(source string) (apm.Looker, error)
	GetStrategyRunner(name string) (strategy.Runner, error)
}

type limiter interface {
	GetSlot(ctx context.Context, p *sdk.ScalingPolicy) error
	ReleaseSlot(p *sdk.ScalingPolicy)
}

type checker interface {
	RunCheckAndCapCount(ctx context.Context, currentCount int64) (sdk.ScalingAction, error)
	Group() string
}

// Handler monitors a policy for changes and controls when them are sent for
// evaluation.
type Handler struct {
	log hclog.Logger

	// mutators is a list of mutations to apply to policies.
	mutators []Mutator

	policyLock sync.RWMutex
	policy     *sdk.ScalingPolicy

	checkRunners []checker

	targetController targetpkg.Controller

	limiter limiter

	// errCh is used to surface errors while executing the policy
	errChn chan<- error
	// ch is used to listen for policy updates.
	updatesCh <-chan *sdk.ScalingPolicy

	// ticker controls the frequency the policy is sent for evaluation.
	ticker *time.Ticker

	stateLock sync.RWMutex
	state     handlerState

	actionLock sync.RWMutex
	nextAction sdk.ScalingAction

	cooldownLock    sync.RWMutex
	outOfCooldownOn time.Time

	pm dependencyGetter

	// Ent only field
	evaluateAfter       time.Duration
	historicalAPMGetter HistoricalAPMGetter
}

type HandlerConfig struct {
	UpdatesChan         chan *sdk.ScalingPolicy
	ErrChan             chan<- error
	Policy              *sdk.ScalingPolicy
	Log                 hclog.Logger
	TargetController    targetpkg.Controller
	Limiter             *Limiter
	DependencyGetter    dependencyGetter
	HistoricalAPMGetter HistoricalAPMGetter
	EvaluateAfter       time.Duration
}

func NewPolicyHandler(config HandlerConfig) (*Handler, error) {

	h := &Handler{
		log: config.Log,
		mutators: []Mutator{
			NomadAPMMutator{},
		},
		pm:                  config.DependencyGetter,
		historicalAPMGetter: config.HistoricalAPMGetter,
		evaluateAfter:       config.EvaluateAfter,
		targetController:    config.TargetController,
		updatesCh:           config.UpdatesChan,
		policy:              config.Policy,
		errChn:              config.ErrChan,
		limiter:             config.Limiter,
		stateLock:           sync.RWMutex{},
		state:               StateIdle,
	}

	err := h.loadCheckRunners()
	if err != nil {
		return nil, fmt.Errorf("unable to load the checks for the handler: %w", err)
	}

	currentStatus, err := h.runTargetStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get target status: %v", err)
	}

	// A nil status indicates the target doesn't exist or is not ready, log and
	// continue
	if currentStatus == nil {
		h.log.Warn("target not found", "target", h.policy.Target.Name)
	}

	lastEventTS, err := checkForOutOfBandEvents(currentStatus)
	if err != nil {
		h.log.Warn("unable to get out of band event", "target", h.policy.Target.Name,
			"error", err)
	}

	if lastEventTS > 0 {
		// For out of band events, it is impossible to determine the direction
		// of the last action, assume the shortest period for responsiveness.
		h.updateState(StateCooldown)

		rcd := calculateRemainingCooldown(h.policy.CooldownOnScaleUp,
			nowFunc().UTC().UnixNano(), lastEventTS)
		time.AfterFunc(rcd, func() {
			h.updateState(StateIdle)
		})
	}

	return h, nil
}

// Convert the last event string.
func checkForOutOfBandEvents(status *sdk.TargetStatus) (int64, error) {
	// If the target status includes a last event meta key, check for cooldown
	// due to out-of-band events. This is also useful if the Autoscaler has
	// been re-deployed.
	if status.Meta == nil {
		return 0, nil
	}

	ts, ok := status.Meta[sdk.TargetStatusMetaKeyLastEvent]
	if !ok {
		return 0, nil
	}

	return strconv.ParseInt(ts, 10, 64)
}

func (h *Handler) loadCheckRunners() error {
	runners := []checker{}

	switch h.policy.Type {
	case sdk.ScalingPolicyTypeCluster, sdk.ScalingPolicyTypeHorizontal:
		for _, check := range h.policy.Checks {

			s, err := h.pm.GetStrategyRunner(check.Strategy.Name)
			if err != nil {
				return fmt.Errorf("failed to get strategy %s: %w", check.Strategy.Name, err)
			}

			mg, err := h.pm.GetAPMLooker(check.Source)
			if err != nil {
				return fmt.Errorf("failed to get APM for strategy %s: %w", check.Strategy.Name, err)
			}

			runner := NewCheckRunner(&CheckRunnerConfig{
				Log: h.log.Named("check_handler").With("check", check.Name,
					"source", check.Source, "strategy", check.Strategy.Name),
				StrategyRunner: s,
				MetricsGetter:  mg,
				Policy:         h.policy,
			}, check)

			runners = append(runners, runner)

		}

	case sdk.ScalingPolicyTypeVerticalCPU, sdk.ScalingPolicyTypeVerticalMem:
		runner, err := h.loadVerticalCheckRunner()
		if err != nil {
			return fmt.Errorf("failed to load vertical check %s: %w", h.policy.Type, err)
		}

		runners = append(runners, runner)
	}

	// Do the update as a single operation to avoid partial updates.
	h.checkRunners = runners
	return nil
}

//	Run starts the handler for the given policy
//
// This function blocks until the context provided is canceled.
func (h *Handler) Run(ctx context.Context) {
	h.ticker = time.NewTicker(h.policy.EvaluationInterval)
	defer h.ticker.Stop()

	h.log.Trace("starting policy handler")

	for {
		select {
		case <-ctx.Done():
			h.log.Error("stopping policy handler due to context done")
			return

		case updatedPolicy := <-h.updatesCh:
			// The policy can be nil if the channel is closed meaning the
			// handler is being removed, let the context cancelation take
			// care of returning.
			if updatedPolicy == nil {
				continue
			}

			h.applyMutators(updatedPolicy)
			h.updateHandler(updatedPolicy)

		case <-h.ticker.C:

			currentStatus, err := h.runTargetStatus()
			if err != nil {
				h.errChn <- fmt.Errorf("handler: failed to get target status for target: %s, %w",
					h.policy.Target.Name, err)
				continue
			}

			// A nil status indicates the target doesn't exist, log and return.
			if currentStatus == nil {
				h.errChn <- errTargetNotFound
				continue
			}

			if currentStatus != nil && !currentStatus.Ready {
				h.log.Debug("skipping evaluation, target not ready")
				continue
			}

			currentCount := currentStatus.Count
			action, err := h.calculateNewCount(ctx, currentCount)
			if err != nil {
				h.errChn <- fmt.Errorf("handler: unable to execute policy ID: %s, %w",
					h.policy.ID, err)
				continue
			}

			h.log.Info("calculating scaling target", "policy_id", h.policy.ID,
				"from", currentCount, "to",
				action.Count, "reason", action.Reason, "meta", action.Meta)

			if !scalingNeeded(action, currentCount) {
				h.log.Info("skipping scaling, no action needed")
				continue
			}

			h.updateNextAction(action)

			if nowFunc().After(h.getOutOfCooldownOn()) &&
				h.getState() == StateCooldown {
				h.updateState(StateIdle)
			}

			switch h.getState() {
			case StateCooldown:
				h.log.Info("skipping scaling, policy still on cooldown", "remaining",
					time.Until(h.getOutOfCooldownOn()))

			case StateWaitingTurn:
				h.log.Debug("updating action, waiting to execute")

			case StateScaling:
				h.log.Info("skipping scaling, target still scaling")

			case StateIdle:
				h.log.Debug("requesting slot")
				h.updateState(StateWaitingTurn)

				go func() {
					err := h.waitAndScale(ctx)
					if err != nil {
						h.updateState(StateIdle)
						h.log.Error("unable to scale target", "reason", err)
					}
				}()
			}
		}
	}
}

func scalingNeeded(a sdk.ScalingAction, countCount int64) bool {
	// The DAS returns count but the direction is none, for vertical and horizontal
	// policies checking the direction is enough.
	return (a.Direction == sdk.ScaleDirectionNone && countCount != a.Count) ||
		a.Direction != sdk.ScaleDirectionNone
}

func (h *Handler) waitAndScale(ctx context.Context) error {
	err := h.limiter.GetSlot(ctx, h.policy)
	if err != nil {
		return fmt.Errorf("timeout waiting for execution time: %w", err)
	}

	defer h.limiter.ReleaseSlot(h.policy)

	h.log.Debug("slot granted")

	action := h.getNextAction()
	h.updateState(StateScaling)

	// Measure how long it takes to invoke the scaling actions. This helps
	// understand the time taken to interact with the remote target and action
	// the scaling action.
	labels := []metrics.Label{
		{Name: "policy_id", Value: h.policy.ID},
		{Name: "target_name", Value: h.policy.Target.Name},
	}
	defer metrics.MeasureSinceWithLabels([]string{"scale", "invoke_ms"},
		nowFunc(), labels)

	err = h.runTargetScale(action)
	if err != nil {
		return fmt.Errorf("failed to get scale target: %w", err)
	}

	h.updateState(StateCooldown)
	cd := calculateCooldown(h.policy, action)
	h.updateOutOfCooldownOn(nowFunc().Add(cd))
	h.log.Debug("successfully submitted scaling action to target, scaling policy has been placed into cooldown",
		"desired_count", action.Count, "cooldown", cd)

	return nil
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

	// Clear existing check handlers and load new ones.
	h.checkRunners = nil

	err := h.loadCheckRunners()
	if err != nil {
		h.errChn <- fmt.Errorf("unable to update policy, failed to load check handlers: %w", err)
		return
	}

	h.log.Debug("check handlers updated", "count", len(h.checkRunners))
	h.policy = updatedPolicy
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

// calculateNewCount is the main part of the controller, it
// gets the metrics and the necessary new count to keep up with the policy
// and generates a scaling action if needed.
func (h *Handler) calculateNewCount(ctx context.Context, currentCount int64) (sdk.ScalingAction, error) {
	h.log.Debug("received policy for evaluation")

	// Record the start time of the eval portion of this function. The labels
	// are also used across multiple metrics, so define them.
	evalStartTime := nowFunc()
	labels := []metrics.Label{
		{Name: "policy_id", Value: h.policy.ID},
		{Name: "target_name", Value: h.policy.Target.Name},
	}

	// Store check results by group so we can compare their results together.
	checkGroups := make(map[string][]checkResult)

	for _, ch := range h.checkRunners {
		action, err := ch.RunCheckAndCapCount(ctx, currentCount)
		if err != nil {
			return sdk.ScalingAction{}, fmt.Errorf("failed get count from metrics: %v", err)

		}

		g := ch.Group()
		checkGroups[g] = append(checkGroups[g], checkResult{
			action:  &action,
			handler: ch,
			group:   g,
		})
	}

	winner := pickWinnerActionFromGroups(checkGroups)
	if winner.handler == nil || winner.action == nil {
		return sdk.ScalingAction{}, nil
	}

	h.log.Debug("check selected", "direction", winner.action.Direction,
		"count", winner.action.Count)

	// At this point the checks have finished. Therefore emit of metric data
	// tracking how long it takes to run all the checks within a policy.
	metrics.MeasureSinceWithLabels([]string{"scale", "evaluate_ms"}, evalStartTime, labels)

	if winner.action.Count == sdk.StrategyActionMetaValueDryRunCount {
		h.log.Debug("registering scaling event",
			"count", currentCount, "reason", winner.action.Reason, "meta", winner.action.Meta)
	}

	return *winner.action, nil
}

// runTargetStatus wraps the target.Status call to provide operational
// functionality.
func (h *Handler) runTargetStatus() (*sdk.TargetStatus, error) {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.policy.Target.Name}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), labels)

	return h.targetController.Status(h.policy.Target.Config)
}

// runTargetScale wraps the target.Scale call to provide operational
// functionality.
func (h *Handler) runTargetScale(action sdk.ScalingAction) error {

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := h.policy.Target.Config["dry-run"]; ok && val == "true" {
		h.log.Info("scaling dry-run is enabled, using no-op task group count")
		action.SetDryRun()
	}

	labels := []metrics.Label{
		{Name: "policy_id", Value: h.policy.ID},
		{Name: "target_name", Value: h.policy.Target.Name},
		{Name: "plugin_name", Value: h.policy.Target.Name},
	}

	// Trigger a metric measure to track latency of the call.
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), labels)
	h.log.Debug("scaling target", "target", h.policy.Target.Name, "count", action.Count)

	err := h.targetController.Scale(action, h.policy.Target.Config)
	if err != nil {
		metrics.IncrCounterWithLabels([]string{"scale", "invoke", "error_count"}, 1, labels)
		return err
	}
	metrics.IncrCounterWithLabels([]string{"scale", "invoke", "success_count"}, 1, labels)

	return nil
}

func calculateCooldown(p *sdk.ScalingPolicy, a sdk.ScalingAction) time.Duration {
	if a.Direction == sdk.ScaleDirectionUp {
		return p.CooldownOnScaleUp
	}

	return p.Cooldown
}

// pickWinnerActionFromGroups decide which action wins in the group.
// The decision processes still picks the safest choice, but it handles `none`
//
//	actions a little differently.
//
// Since grouped checks have corelated metrics, it's expected that most
// checks will result in `none` actions as the data will be somewhere
// else. So we ignore none actions unless _all_ checks in the group
// vote for `none` to avoid accidentally scaling down when comparing
// with other groups.
func pickWinnerActionFromGroups(checkGroups map[string][]checkResult) checkResult {
	var winner checkResult
	for group, results := range checkGroups {

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
			continue
		}

		winner = winner.preempt(groupWinner)
	}

	return winner
}

func (h *Handler) getState() handlerState {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	return h.state
}

func (h *Handler) updateState(hs handlerState) {
	h.stateLock.Lock()
	defer h.stateLock.Unlock()

	h.state = hs
}

func (h *Handler) getNextAction() sdk.ScalingAction {
	h.actionLock.RLock()
	defer h.actionLock.RUnlock()

	return h.nextAction
}

func (h *Handler) updateNextAction(a sdk.ScalingAction) {
	h.actionLock.Lock()
	defer h.actionLock.Unlock()

	h.nextAction = a
}

func (h *Handler) getOutOfCooldownOn() time.Time {
	h.cooldownLock.RLock()
	defer h.cooldownLock.RUnlock()

	return h.outOfCooldownOn
}

func (h *Handler) updateOutOfCooldownOn(ooc time.Time) {
	h.cooldownLock.Lock()
	defer h.cooldownLock.Unlock()

	h.outOfCooldownOn = ooc
}

// calculateRemainingCooldown calculates the remaining cooldown based on the
// time since the last event. The remaining period can be negative, indicating
// no cooldown period is required.
func calculateRemainingCooldown(cd time.Duration, ts, lastEvent int64) time.Duration {
	return cd - time.Duration(ts-lastEvent)
}

type checkResult struct {
	action  *sdk.ScalingAction
	handler checker
	group   string
}

func (c checkResult) preempt(other checkResult) checkResult {
	winner := sdk.PreemptScalingAction(c.action, other.action)
	if winner == c.action {
		return c
	}
	return other
}
