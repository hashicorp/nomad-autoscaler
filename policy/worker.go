package policy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// errTargetNotReady is used by a check handler to indicate the policy target
// is not ready.
var errTargetNotReady = errors.New("target not ready")

// Worker is responsible for executing a policy evaluation request.
type Worker struct {
	logger        hclog.Logger
	pluginManager *manager.PluginManager
	policyManager *Manager
}

// NewWorker returns a new Worker instance.
func NewWorker(l hclog.Logger, pm *manager.PluginManager, m *Manager) *Worker {
	return &Worker{
		logger:        l.Named("worker"),
		pluginManager: pm,
		policyManager: m,
	}
}

// HandlePolicy evaluates a policy and execute a scaling action if necessary.
func (w *Worker) HandlePolicy(ctx context.Context, eval *sdk.ScalingEvaluation) {

	// Record the start time of the eval portion of this function. The labels
	// are also used across multiple metrics, so define them.
	evalStartTime := time.Now()
	labels := []metrics.Label{
		{Name: "policy_id", Value: eval.Policy.ID},
		{Name: "target_name", Value: eval.Policy.Target.Name},
	}

	logger := w.logger.With("policy_id", eval.Policy.ID, "target", eval.Policy.Target.Name)
	checks := make(map[string]*checkHandler)

	logger.Info("received policy for evaluation")

	handlersCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start check handlers.
	for _, checkEval := range eval.CheckEvaluations {
		checkHandler := newCheckHandler(logger, eval.Policy, checkEval, w.pluginManager)
		checks[checkEval.Check.Name] = checkHandler
		go checkHandler.start(handlersCtx)
	}

	// winningAction is the action to be executed after all checks' results are
	// reconciled.
	var winningAction *sdk.ScalingAction
	var winningHandler *checkHandler

	// Initial results should return fairly quickly.
	// Timeout if it is taking too long.
	resultsTimeout := time.NewTimer(5 * time.Minute)

	// Wait for check results and pick the winner.
	for check, handler := range checks {
		select {
		case <-ctx.Done():
			logger.Info("policy evaluation canceled")
			return
		case <-resultsTimeout.C:
			logger.Warn("timeout while waiting for policy check results")
			return
		case r := <-handler.results():
			if r.err != nil {
				if r.err == errTargetNotReady {
					logger.Info("target not ready")
					return
				}

				// TODO(luiz): properly handle errors.
				logger.Warn("failed to evaluate check", "error", r.err, "check", check)
				continue
			}

			winningAction = sdk.PreemptScalingAction(winningAction, r.action)
			if winningAction == r.action {
				winningHandler = handler
			}
		}
	}

	// Stop and drain results timeout timer.
	if !resultsTimeout.Stop() {
		<-resultsTimeout.C
	}

	// At this point the checks have finished. Therefore emit of metric data
	// tracking how long it takes to run all the checks within a policy.
	metrics.MeasureSinceWithLabels([]string{"scale", "evaluate_ms"}, evalStartTime, labels)

	if winningHandler == nil || winningAction.Direction == sdk.ScaleDirectionNone {
		logger.Info("no checks need to be executed")
		return
	}

	logger.Trace(fmt.Sprintf("check %s selected", winningHandler.checkEval.Check.Name),
		"direction", winningAction.Direction, "count", winningAction.Count)

	// Unblock winning handler and cancel the others. The default guards
	// against the possibility of there being no receiver on the proceedCh.
	for _, handler := range checks {
		select {
		case handler.proceedCh <- handler == winningHandler:
		default:
		}
	}

	// Measure how long it takes to invoke the scaling actions. This helps
	// understand the time taken to interact with the remote target and action
	// the scaling action.
	defer metrics.MeasureSinceWithLabels([]string{"scale", "invoke_ms"}, time.Now(), labels)

	// Block until winning handler returns.
	select {
	case <-ctx.Done():
		logger.Info("policy evaluation canceled")
		return
	case r := <-winningHandler.results():
		if r.err != nil {
			logger.Error("failed to execute check",
				"error", r.err, "check", winningHandler.checkEval.Check.Name)
			return
		}
		if r.action == nil {
			return
		}
	}

	// Enforce the cooldown after a successful scaling event.
	w.policyManager.EnforceCooldown(eval.Policy.ID, eval.Policy.Cooldown)

	logger.Info("policy evaluation complete")
}

// checkHandler evaluates one of the checks of a policy.
type checkHandler struct {
	logger        hclog.Logger
	policy        *sdk.ScalingPolicy
	checkEval     *sdk.ScalingCheckEvaluation
	pluginManager *manager.PluginManager
	resultCh      chan checkHandlerResult
	proceedCh     chan bool
}

type checkHandlerResult struct {
	action *sdk.ScalingAction
	err    error
}

// newCheckHandler returns a new checkHandler instance.
func newCheckHandler(l hclog.Logger, p *sdk.ScalingPolicy, c *sdk.ScalingCheckEvaluation, pm *manager.PluginManager) *checkHandler {
	return &checkHandler{
		logger: l.Named("check_handler").With(
			"check", c.Check.Name,
			"source", c.Check.Source,
			"strategy", c.Check.Strategy.Name,
		),
		policy:        p,
		checkEval:     c,
		pluginManager: pm,
		resultCh:      make(chan checkHandlerResult),
		proceedCh:     make(chan bool),
	}
}

// results returns a read-only version of resultCh.
func (h *checkHandler) results() <-chan checkHandlerResult {
	return h.resultCh
}

// start begins the execution of the check handler.
//
// The process is split in two phases:
//
//   1) check evaluation
//      The check handler uses the plugins defined in the policy and the check
//      to calculate what the next scaling action should be (if any).
//
//   2) action execution
//      If a scaling action is necessary, the check handler will call the
//      target plugin to trigger a scaling event.
//
// Since there are multiple checks in a policy, after step 1 the action is
// sent back to the worker and the handler halts until an action is selected
// for execution. The check handler that produced the selected action is
// allowed to continue to step 2 while the others are cancelled.

func (h *checkHandler) start(ctx context.Context) {
	defer close(h.resultCh)

	h.logger.Info("received policy check for evaluation")

	result := checkHandlerResult{}

	var targetInst target.Target
	var apmInst apm.APM
	var strategyInst strategy.Strategy

	// Dispense plugins.
	targetPlugin, err := h.pluginManager.Dispense(h.policy.Target.Name, plugins.PluginTypeTarget)
	if err != nil {
		result.err = fmt.Errorf(`target plugin "%s" not initialized: %v`, h.policy.Target.Name, err)
		h.resultCh <- result
		return
	}
	targetInst = targetPlugin.Plugin().(target.Target)

	apmPlugin, err := h.pluginManager.Dispense(h.checkEval.Check.Source, plugins.PluginTypeAPM)
	if err != nil {
		result.err = fmt.Errorf(`apm plugin "%s" not initialized: %v`, h.checkEval.Check.Source, err)
		h.resultCh <- result
		return
	}
	apmInst = apmPlugin.Plugin().(apm.APM)

	strategyPlugin, err := h.pluginManager.Dispense(h.checkEval.Check.Strategy.Name, plugins.PluginTypeStrategy)
	if err != nil {
		result.err = fmt.Errorf(`strategy plugin "%s" not initialized: %v`, h.checkEval.Check.Strategy.Name, err)
		h.resultCh <- result
		return
	}
	strategyInst = strategyPlugin.Plugin().(strategy.Strategy)

	// Fetch target status.
	currentStatus, err := h.runTargetStatus(targetInst)
	if err != nil {
		result.err = fmt.Errorf("failed to fetch current count: %v", err)
		h.resultCh <- result
		return
	}
	if !currentStatus.Ready {
		result.err = errTargetNotReady
		h.resultCh <- result
		return
	}

	// Query check's APM.
	h.checkEval.Metrics, err = h.runAPMQuery(apmInst)
	if err != nil {
		result.err = fmt.Errorf("failed to query source: %v", err)
		h.resultCh <- result
		return
	}

	// Make sure metrics are sorted consistently.
	sort.Sort(h.checkEval.Metrics)

	if len(h.checkEval.Metrics) == 0 {
		h.logger.Info("no metrics available")
		return
	}

	// Calculate new count using check's Strategy.
	h.logger.Info("calculating new count", "count", currentStatus.Count)
	runResp, err := h.runStrategyRun(strategyInst, currentStatus.Count)
	if err != nil {
		result.err = fmt.Errorf("failed to execute strategy: %v", err)
		h.resultCh <- result
		return
	}
	h.checkEval = runResp

	if h.checkEval.Action.Direction == sdk.ScaleDirectionNone {
		// Make sure we are currently within [min, max] limits even if there's
		// no action to execute
		var minMaxAction *sdk.ScalingAction

		if currentStatus.Count < h.policy.Min {
			minMaxAction = &sdk.ScalingAction{
				Count:     h.policy.Min,
				Direction: sdk.ScaleDirectionUp,
				Reason:    fmt.Sprintf("current count (%d) below limit (%d)", currentStatus.Count, h.policy.Min),
			}
		} else if currentStatus.Count > h.policy.Max {
			minMaxAction = &sdk.ScalingAction{
				Count:     h.policy.Max,
				Direction: sdk.ScaleDirectionDown,
				Reason:    fmt.Sprintf("current count (%d) above limit (%d)", currentStatus.Count, h.policy.Max),
			}
		}

		if minMaxAction != nil {
			h.checkEval.Action = minMaxAction
		} else {
			h.logger.Info("nothing to do")
			result.action = &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}
			h.resultCh <- result
			return
		}
	}

	// Make sure new count value is within [min, max] limits
	h.checkEval.Action.CapCount(h.policy.Min, h.policy.Max)

	// Skip action if count doesn't change.
	if currentStatus.Count == h.checkEval.Action.Count {
		h.logger.Info("nothing to do", "from", currentStatus.Count, "to", h.checkEval.Action.Count)

		result.action = &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}
		h.resultCh <- result
		return
	}

	result.action = h.checkEval.Action

	// Send result back and wait to see if we should proceed.
	h.resultCh <- result
	select {
	case <-ctx.Done():
		return
	case proceed := <-h.proceedCh:
		if !proceed {
			h.logger.Debug("check not selected")
			return
		}
	}

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := h.policy.Target.Config["dry-run"]; ok && val == "true" {
		h.logger.Info("scaling dry-run is enabled, using no-op task group count")
		h.checkEval.Action.SetDryRun()
	}

	if h.checkEval.Action.Count == sdk.StrategyActionMetaValueDryRunCount {
		h.logger.Info("registering scaling event",
			"count", currentStatus.Count, "reason", h.checkEval.Action.Reason, "meta", h.checkEval.Action.Meta)
	} else {
		h.logger.Info("scaling target",
			"from", currentStatus.Count, "to", h.checkEval.Action.Count,
			"reason", h.checkEval.Action.Reason, "meta", h.checkEval.Action.Meta)
	}

	// Scale the target. If we receive an error add this onto the result so the
	// handler understand what do to.
	if err = h.runTargetScale(targetInst, *h.checkEval.Action); err != nil {
		result.err = fmt.Errorf("failed to scale target: %v", err)
		h.logger.Error("failed to submit scaling action to target", "error", err)
		metrics.IncrCounter([]string{"scale", "invoke", "error_count"}, 1)
	} else {
		h.logger.Info("successfully submitted scaling action to target",
			"desired_count", h.checkEval.Action.Count)
		metrics.IncrCounter([]string{"scale", "invoke", "success_count"}, 1)
	}

	// Ensure we send a result otherwise the Worker.HandlePolicy routine will
	// leak waiting endlessly for the result it will never receive, poor thing.
	h.resultCh <- result
}

// runTargetStatus wraps the target.Status call to provide operational
// functionality.
func (h *checkHandler) runTargetStatus(targetImpl target.Target) (*sdk.TargetStatus, error) {

	h.logger.Info("fetching current count")

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.policy.Target.Name}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), labels)

	return targetImpl.Status(h.policy.Target.Config)
}

// runTargetScale wraps the target.Scale call to provide operational
// functionality.
func (h *checkHandler) runTargetScale(targetImpl target.Target, action sdk.ScalingAction) error {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.policy.Target.Name}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), labels)

	return targetImpl.Scale(action, h.policy.Target.Config)
}

// runAPMQuery wraps the apm.Query call to provide operational functionality.
func (h *checkHandler) runAPMQuery(apmImpl apm.APM) (sdk.TimestampedMetrics, error) {

	h.logger.Info("querying source", "query", h.checkEval.Check.Query, "source", h.checkEval.Check.Source)

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.checkEval.Check.Source}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "apm", "query", "invoke_ms"}, time.Now(), labels)

	// Calculate query range from the query window defined in the check.
	to := time.Now()
	from := to.Add(-h.checkEval.Check.QueryWindow)
	r := sdk.TimeRange{From: from, To: to}

	return apmImpl.Query(h.checkEval.Check.Query, r)
}

// runStrategyRun wraps the strategy.Run call to provide operational functionality.
func (h *checkHandler) runStrategyRun(strategyImpl strategy.Strategy, count int64) (*sdk.ScalingCheckEvaluation, error) {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: h.checkEval.Check.Strategy.Name},
		{Name: "policy_id", Value: h.policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "strategy", "run", "invoke_ms"}, time.Now(), labels)

	return strategyImpl.Run(h.checkEval, count)
}
