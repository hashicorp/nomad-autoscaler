package policyeval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
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
	pipeline      Pipeline
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
		pipeline:      NewBasePipeline(l, pm),
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

	for _, h := range eval.Policy.Hooks {
		if err := h.PreStart(eval); err != nil {
			return err
		}
	}

	// Run status stage.
	err := w.pipeline.Status(eval)
	if err != nil {
		return fmt.Errorf("failed to fetch current count: %v", err)
	}
	if !eval.TargetStatus.Ready {
		return errTargetNotReady
	}

	for _, h := range eval.Policy.Hooks {
		if err := h.PostStatus(eval); err != nil {
			return err
		}
	}

	// Run query stage.
	err = w.pipeline.Query(eval)
	if err != nil {
		return fmt.Errorf("failed to query APM: %v", err)
	}

	for _, h := range eval.Policy.Hooks {
		if err := h.PostQuery(eval); err != nil {
			return err
		}
	}

	// Run strategy stage.
	err = w.pipeline.Strategy(eval)
	if err != nil {
		return fmt.Errorf("failed to run strategy: %v", err)
	}

	for _, h := range eval.Policy.Hooks {
		if err := h.PostStrategy(eval); err != nil {
			return err
		}
	}

	// At this point the checks have finished. Therefore emit of metric data
	// tracking how long it takes to run all the checks within a policy.
	metrics.MeasureSinceWithLabels([]string{"scale", "evaluate_ms"}, evalStartTime, labels)

	// Decide which action to execute.
	action := w.reconcileCheckResults(eval)
	if action == nil {
		logger.Debug("no checks need to be executed")
		return nil
	}
	// Store winning action into the policy evaluation so it's available to
	// the next stages and hooks.
	eval.Action = action

	// Last check for early exit before scaling the target, which we consider
	// a non-preemptable action since we cannot be sure that a scaling action can
	// be cancelled halfway through or undone.
	select {
	case <-ctx.Done():
		w.logger.Info("stopping worker")
		return nil
	default:
	}

	// Measure how long it takes to invoke the scaling actions. This helps
	// understand the time taken to interact with the remote target and action
	// the scaling action.
	defer metrics.MeasureSinceWithLabels([]string{"scale", "invoke_ms"}, time.Now(), labels)

	for _, h := range eval.Policy.Hooks {
		if err := h.PreScale(eval); err != nil {
			return err
		}
	}

	// Run the scale stage.
	err = w.pipeline.Scale(eval)
	if err != nil {
		return fmt.Errorf("failed to scale target: %v", err)
	}

	for _, h := range eval.Policy.Hooks {
		if err := h.PostScale(eval); err != nil {
			return err
		}
	}

	logger.Info("successfully submitted scaling action to target", "desired_count", eval.Action.Count)
	logger.Info("policy evaluation complete")

	return nil
}

func (w *BaseWorker) reconcileCheckResults(eval *sdk.ScalingEvaluation) *sdk.ScalingAction {
	logger := w.logger.With("policy_id", eval.Policy.ID, "target", eval.Policy.Target.Name)

	// winningCheck is the check to be executed after all checks' results are
	// reconciled.
	var winningCheck *sdk.ScalingCheckEvaluation

	for _, checkEval := range eval.CheckEvaluations {
		if winningCheck == nil {
			winningCheck = checkEval
			continue
		}

		winningAction := sdk.PreemptScalingAction(winningCheck.Action, checkEval.Action)
		if winningAction == checkEval.Action {
			winningCheck = checkEval
		}
	}

	if winningCheck == nil || winningCheck.Action == nil || winningCheck.Action.Direction == sdk.ScaleDirectionNone {
		return nil
	}

	logger.Trace(fmt.Sprintf("check %s selected", winningCheck.Check.Name),
		"direction", winningCheck.Action.Direction, "count", winningCheck.Action.Count)

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := eval.Policy.Target.Config["dry-run"]; ok && val == "true" {
		logger.Info("scaling dry-run is enabled, using no-op task group count")
		winningCheck.Action.SetDryRun()
	}

	if winningCheck.Action.Count == sdk.StrategyActionMetaValueDryRunCount {
		logger.Debug("registering scaling event",
			"count", eval.TargetStatus.Count, "reason", winningCheck.Action.Reason, "meta", winningCheck.Action.Meta)
	} else {
		logger.Info("scaling target",
			"from", eval.TargetStatus.Count, "to", winningCheck.Action.Count,
			"reason", winningCheck.Action.Reason, "meta", winningCheck.Action.Meta)
	}

	return winningCheck.Action
}
