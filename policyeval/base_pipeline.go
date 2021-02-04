package policyeval

import (
	"fmt"
	"sort"
	"time"

	"github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type BasePipeline struct {
	logger        hclog.Logger
	pluginManager *manager.PluginManager
}

func NewBasePipeline(l hclog.Logger, pm *manager.PluginManager) *BasePipeline {
	return &BasePipeline{
		logger:        l,
		pluginManager: pm,
	}
}

func (p *BasePipeline) Status(eval *sdk.ScalingEvaluation) error {
	// Dispense taget plugin.
	targetPlugin, err := p.pluginManager.Dispense(eval.Policy.Target.Name, plugins.PluginTypeTarget)
	if err != nil {
		return fmt.Errorf(`target plugin "%s" not initialized: %v`, eval.Policy.Target.Name, err)
	}
	targetInst, ok := targetPlugin.Plugin().(target.Target)
	if !ok {
		return fmt.Errorf(`"%s" is not a target plugin`, eval.Policy.Target.Name)
	}

	// Fetch target status.
	p.logger.Debug("fetching current count")

	// Record metric.
	labels := []metrics.Label{{Name: "plugin_name", Value: eval.Policy.Target.Name}, {Name: "policy_id", Value: eval.Policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), labels)

	currentStatus, err := targetInst.Status(eval.Policy.Target.Config)
	if err != nil {
		return fmt.Errorf("failed to fetch current count: %v", err)
	}

	// Update eval.
	eval.TargetStatus = currentStatus
	return nil
}

func (p *BasePipeline) Query(eval *sdk.ScalingEvaluation) error {
	for _, checkEval := range eval.CheckEvaluations {
		result, err := p.queryAPM(checkEval, eval)
		if err != nil {
			return err
		}
		checkEval.Metrics = result
	}

	return nil
}

func (p *BasePipeline) Strategy(eval *sdk.ScalingEvaluation) error {
	for _, checkEval := range eval.CheckEvaluations {
		// Skip check if there are no metrics available.
		if len(checkEval.Metrics) == 0 {
			p.logger.Warn("no metrics available")
			checkEval.Action = &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}
			continue
		}

		action, err := p.runStrategy(checkEval, eval)
		if err != nil {
			return err
		}
		checkEval.Action = action
	}

	return nil
}

func (p *BasePipeline) Scale(eval *sdk.ScalingEvaluation) error {
	// Dispense taget plugin.
	targetPlugin, err := p.pluginManager.Dispense(eval.Policy.Target.Name, plugins.PluginTypeTarget)
	if err != nil {
		return fmt.Errorf(`target plugin "%s" not initialized: %v`, eval.Policy.Target.Name, err)
	}
	targetInst, ok := targetPlugin.Plugin().(target.Target)
	if !ok {
		return fmt.Errorf(`"%s" is not a target plugin`, eval.Policy.Target.Name)
	}

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: eval.Policy.Target.Name},
		{Name: "policy_id", Value: eval.Policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), labels)

	err = targetInst.Scale(*eval.Action, eval.Policy.Target.Config)
	if err != nil {
		metrics.IncrCounter([]string{"scale", "invoke", "error_count"}, 1)
		return fmt.Errorf("failed to scale target: %v", err)
	}

	metrics.IncrCounter([]string{"scale", "invoke", "success_count"}, 1)
	return nil
}

func (p *BasePipeline) queryAPM(checkEval *sdk.ScalingCheckEvaluation, eval *sdk.ScalingEvaluation) (sdk.TimestampedMetrics, error) {
	p.logger.Debug("querying source", "query", checkEval.Check.Query, "source", checkEval.Check.Source)

	// Dispense APM plugin.
	apmPlugin, err := p.pluginManager.Dispense(checkEval.Check.Source, plugins.PluginTypeAPM)
	if err != nil {
		return nil, fmt.Errorf(`apm plugin "%s" not initialized: %v`, checkEval.Check.Source, err)
	}
	apmInst, ok := apmPlugin.Plugin().(apm.APM)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not an APM plugin`, checkEval.Check.Source)
	}

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: checkEval.Check.Source},
		{Name: "policy_id", Value: eval.Policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "apm", "query", "invoke_ms"}, time.Now(), labels)

	// Calculate query range from the query window defined in the check.
	to := time.Now()
	from := to.Add(-checkEval.Check.QueryWindow)
	r := sdk.TimeRange{From: from, To: to}

	// Run query.
	results, err := apmInst.Query(checkEval.Check.Query, r)
	if err != nil {
		return nil, err
	}

	// Make sure metrics are sorted consistently.
	sort.Sort(results)

	return results, nil
}

func (p *BasePipeline) runStrategy(checkEval *sdk.ScalingCheckEvaluation, eval *sdk.ScalingEvaluation) (*sdk.ScalingAction, error) {
	// Calculate new count using check's Strategy.
	p.logger.Debug("calculating new count", "count", eval.TargetStatus.Count)

	// Dispense Strategy plugin.
	strategyPlugin, err := p.pluginManager.Dispense(checkEval.Check.Strategy.Name, plugins.PluginTypeStrategy)
	if err != nil {
		return nil, fmt.Errorf(`strategy plugin "%s" not initialized: %v`, checkEval.Check.Strategy.Name, err)
	}
	strategyInst, ok := strategyPlugin.Plugin().(strategy.Strategy)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not a strategy plugin`, checkEval.Check.Strategy.Name)
	}

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: checkEval.Check.Strategy.Name},
		{Name: "policy_id", Value: eval.Policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "strategy", "run", "invoke_ms"}, time.Now(), labels)

	runResult, err := strategyInst.Run(checkEval, eval.TargetStatus.Count)
	if err != nil {
		return nil, fmt.Errorf("failed to execute strategy: %v", err)
	}
	action := runResult.Action

	if action.Direction == sdk.ScaleDirectionNone {
		// Make sure we are currently within [min, max] limits even if there's
		// no action to execute
		var minMaxAction *sdk.ScalingAction

		if eval.TargetStatus.Count < eval.Policy.Min {
			minMaxAction = &sdk.ScalingAction{
				Count:     eval.Policy.Min,
				Direction: sdk.ScaleDirectionUp,
				Reason:    fmt.Sprintf("current count (%d) below limit (%d)", eval.TargetStatus.Count, eval.Policy.Min),
			}
		} else if eval.TargetStatus.Count > eval.Policy.Max {
			minMaxAction = &sdk.ScalingAction{
				Count:     eval.Policy.Max,
				Direction: sdk.ScaleDirectionDown,
				Reason:    fmt.Sprintf("current count (%d) above limit (%d)", eval.TargetStatus.Count, eval.Policy.Max),
			}
		}

		if minMaxAction != nil {
			action = minMaxAction
		} else {
			p.logger.Debug("nothing to do")
			return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
		}
	}

	// Canonicalize action so plugins don't have to.
	action.Canonicalize()

	// Make sure new count value is within [min, max] limits
	action.CapCount(eval.Policy.Min, eval.Policy.Max)

	// Skip action if count doesn't change.
	if eval.TargetStatus.Count == checkEval.Action.Count {
		p.logger.Debug("nothing to do", "from", eval.TargetStatus.Count, "to", checkEval.Action.Count)
		return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
	}

	return action, nil
}
