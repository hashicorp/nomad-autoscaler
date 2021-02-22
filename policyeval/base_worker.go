package policyeval

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

// errTargetNotReady is used by a check handler to indicate the policy target
// is not ready.
var errTargetNotReady = errors.New("target not ready")

// Worker is responsible for executing a policy evaluation request.
type BaseWorker struct {
	id            string
	logger        hclog.Logger
	pluginManager *manager.PluginManager
	policyManager *policy.Manager
	broker        *Broker
	queue         string
}

// NewBaseWorker returns a new BaseWorker instance.
func NewBaseWorker(l hclog.Logger, pm *manager.PluginManager, m *policy.Manager, b *Broker, queue string) *BaseWorker {
	id := uuid.Generate()

	return &BaseWorker{
		id:            id,
		logger:        l.Named("worker").With("id", id, "queue", queue),
		pluginManager: pm,
		policyManager: m,
		broker:        b,
		queue:         queue,
	}
}

func (w *BaseWorker) Run(ctx context.Context) {
	w.logger.Info("starting worker")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping worker")
			return
		default:
		}

		eval, token, err := w.broker.Dequeue(ctx, w.queue)
		if err != nil {
			w.logger.Warn("failed to dequeue evaluation", "err", err)
			continue
		}

		if eval == nil {
			// Nothing to do for now or we timedout, let's loop.
			continue
		}

		logger := w.logger.With(
			"eval_id", eval.ID,
			"eval_token", token,
			"policy_id", eval.Policy.ID)

		if err := w.handlePolicy(ctx, eval); err != nil {
			logger.Error("failed to evaluate policy", "err", err)

			// Notify broker that policy eval was not successful.
			if err := w.broker.Nack(eval.ID, token); err != nil {
				logger.Warn("failed to NACK policy evaluation", "err", err)
			}
			continue
		}

		// Notify broker that policy eval was successful.
		if err := w.broker.Ack(eval.ID, token); err != nil {
			logger.Warn("failed to ACK policy evaluation", "err", err)
		}
	}
}

// HandlePolicy evaluates a policy and execute a scaling action if necessary.
func (w *BaseWorker) handlePolicy(ctx context.Context, eval *sdk.ScalingEvaluation) error {

	// Record the start time of the eval portion of this function. The labels
	// are also used across multiple metrics, so define them.
	evalStartTime := time.Now()
	labels := []metrics.Label{
		{Name: "policy_id", Value: eval.Policy.ID},
		{Name: "target_name", Value: eval.Policy.Target.Name},
	}

	logger := w.logger.With("policy_id", eval.Policy.ID, "target", eval.Policy.Target.Name)
	logger.Debug("received policy for evaluation")

	// Dispense taget plugin.
	targetPlugin, err := w.pluginManager.Dispense(eval.Policy.Target.Name, sdk.PluginTypeTarget)
	if err != nil {
		return fmt.Errorf(`target plugin "%s" not initialized: %v`, eval.Policy.Target.Name, err)
	}
	targetInst, ok := targetPlugin.Plugin().(target.Target)
	if !ok {
		return fmt.Errorf(`"%s" is not a target plugin`, eval.Policy.Target.Name)
	}

	// Fetch target status.
	logger.Debug("fetching current count")

	currentStatus, err := w.runTargetStatus(targetInst, eval.Policy)
	if err != nil {
		return fmt.Errorf("failed to fetch current count: %v", err)
	}
	if !currentStatus.Ready {
		return errTargetNotReady
	}

	// Prepare handlers.
	handlersCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// winningAction is the action to be executed after all checks' results are
	// reconciled.
	var winningAction *sdk.ScalingAction
	var winningHandler *checkHandler

	// Start check handlers.
	for _, checkEval := range eval.CheckEvaluations {
		checkHandler := newCheckHandler(logger, eval.Policy, checkEval, w.pluginManager)

		// Wrap target status call in a goroutine so we can listen for ctx as well.
		var action *sdk.ScalingAction
		var err error
		doneCh := make(chan interface{})

		go func() {
			defer close(doneCh)
			action, err = checkHandler.start(handlersCtx, currentStatus)
		}()

		select {
		case <-ctx.Done():
			w.logger.Info("stopping worker")
			return nil
		case <-doneCh:
		}

		if err != nil {
			logger.Warn("failed to run check", "err", err)
			continue
		}

		winningAction = sdk.PreemptScalingAction(winningAction, action)
		if winningAction == action {
			winningHandler = checkHandler
		}
	}

	// At this point the checks have finished. Therefore emit of metric data
	// tracking how long it takes to run all the checks within a policy.
	metrics.MeasureSinceWithLabels([]string{"scale", "evaluate_ms"}, evalStartTime, labels)

	if winningHandler == nil || winningAction == nil || winningAction.Direction == sdk.ScaleDirectionNone {
		logger.Debug("no checks need to be executed")
		return nil
	}

	logger.Trace(fmt.Sprintf("check %s selected", winningHandler.checkEval.Check.Name),
		"direction", winningAction.Direction, "count", winningAction.Count)

	// Measure how long it takes to invoke the scaling actions. This helps
	// understand the time taken to interact with the remote target and action
	// the scaling action.
	defer metrics.MeasureSinceWithLabels([]string{"scale", "invoke_ms"}, time.Now(), labels)

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := eval.Policy.Target.Config["dry-run"]; ok && val == "true" {
		logger.Info("scaling dry-run is enabled, using no-op task group count")
		winningAction.SetDryRun()
	}

	if winningAction.Count == sdk.StrategyActionMetaValueDryRunCount {
		logger.Debug("registering scaling event",
			"count", currentStatus.Count, "reason", winningAction.Reason, "meta", winningAction.Meta)
	} else {
		logger.Info("scaling target",
			"from", currentStatus.Count, "to", winningAction.Count,
			"reason", winningAction.Reason, "meta", winningAction.Meta)
	}

	// Last check for early exit before scaling the target, which we consider
	// a non-preemptable action since we cannot be sure that a scaling action can
	// be cancelled halfway through or undone.
	select {
	case <-ctx.Done():
		w.logger.Info("stopping worker")
		return nil
	default:
	}

	// Scale the target. If we receive an error add this onto the result so the
	// handler understand what do to.
	err = w.runTargetScale(targetInst, eval.Policy, *winningAction)
	if err != nil {
		metrics.IncrCounter([]string{"scale", "invoke", "error_count"}, 1)
		return fmt.Errorf("failed to scale target: %v", err)
	} else {
		logger.Info("successfully submitted scaling action to target",
			"desired_count", winningAction.Count)
		metrics.IncrCounter([]string{"scale", "invoke", "success_count"}, 1)
	}

	// Enforce the cooldown after a successful scaling event.
	w.policyManager.EnforceCooldown(eval.Policy.ID, eval.Policy.Cooldown)

	logger.Info("policy evaluation complete")
	return nil
}

// runTargetStatus wraps the target.Status call to provide operational
// functionality.
func (w *BaseWorker) runTargetStatus(targetImpl target.Target, policy *sdk.ScalingPolicy) (*sdk.TargetStatus, error) {
	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: policy.Target.Name}, {Name: "policy_id", Value: policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), labels)

	return targetImpl.Status(policy.Target.Config)
}

// runTargetScale wraps the target.Scale call to provide operational
// functionality.
func (w *BaseWorker) runTargetScale(targetImpl target.Target, policy *sdk.ScalingPolicy, action sdk.ScalingAction) error {
	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: policy.Target.Name}, {Name: "policy_id", Value: policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), labels)

	return targetImpl.Scale(action, policy.Target.Config)
}

// checkHandler evaluates one of the checks of a policy.
type checkHandler struct {
	logger        hclog.Logger
	policy        *sdk.ScalingPolicy
	checkEval     *sdk.ScalingCheckEvaluation
	pluginManager *manager.PluginManager
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
	}
}

// start begins the execution of the check handler.
func (h *checkHandler) start(ctx context.Context, currentStatus *sdk.TargetStatus) (*sdk.ScalingAction, error) {
	h.logger.Debug("received policy check for evaluation")

	var apmInst apm.APM
	var strategyInst strategy.Strategy

	// Dispense plugins.
	apmPlugin, err := h.pluginManager.Dispense(h.checkEval.Check.Source, sdk.PluginTypeAPM)
	if err != nil {
		return nil, fmt.Errorf(`apm plugin "%s" not initialized: %v`, h.checkEval.Check.Source, err)
	}
	apmInst, ok := apmPlugin.Plugin().(apm.APM)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not an APM plugin`, h.checkEval.Check.Source)
	}

	strategyPlugin, err := h.pluginManager.Dispense(h.checkEval.Check.Strategy.Name, sdk.PluginTypeStrategy)
	if err != nil {
		return nil, fmt.Errorf(`strategy plugin "%s" not initialized: %v`, h.checkEval.Check.Strategy.Name, err)
	}
	strategyInst, ok = strategyPlugin.Plugin().(strategy.Strategy)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not a strategy plugin`, h.checkEval.Check.Strategy.Name)
	}

	// Query check's APM.
	// Wrap call in a goroutine so we can listen for ctx as well.
	apmQueryDoneCh := make(chan interface{})
	go func() {
		defer close(apmQueryDoneCh)
		h.checkEval.Metrics, err = h.runAPMQuery(apmInst)
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-apmQueryDoneCh:
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query source: %v", err)
	}

	// Make sure metrics are sorted consistently.
	sort.Sort(h.checkEval.Metrics)

	if len(h.checkEval.Metrics) == 0 {
		h.logger.Warn("no metrics available")
		return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
	}

	for _, m := range h.checkEval.Metrics {
		h.logger.Trace("metric result", "ts", m.Timestamp, "value", m.Value)
	}

	// Calculate new count using check's Strategy.
	h.logger.Debug("calculating new count", "count", currentStatus.Count)
	runResp, err := h.runStrategyRun(strategyInst, currentStatus.Count)
	if err != nil {
		return nil, fmt.Errorf("failed to execute strategy: %v", err)
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
			h.logger.Debug("nothing to do")
			return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
		}
	}

	// Canonicalize action so plugins don't have to.
	h.checkEval.Action.Canonicalize()

	// Make sure new count value is within [min, max] limits
	h.checkEval.Action.CapCount(h.policy.Min, h.policy.Max)

	// Skip action if count doesn't change.
	if currentStatus.Count == h.checkEval.Action.Count {
		h.logger.Debug("nothing to do", "from", currentStatus.Count, "to", h.checkEval.Action.Count)
		return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
	}

	return h.checkEval.Action, nil
}

// runAPMQuery wraps the apm.Query call to provide operational functionality.
func (h *checkHandler) runAPMQuery(apmImpl apm.APM) (sdk.TimestampedMetrics, error) {

	h.logger.Debug("querying source", "query", h.checkEval.Check.Query, "source", h.checkEval.Check.Source)

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
