// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"fmt"
	"sort"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type coolDownEnforcer interface {
	EnforceCooldown(t time.Duration)
}

type CheckHandlerConfig struct {
	// Log is the logger to use for logging.
	Log hclog.Logger

	// CoolDownEnforcer is the cooldown enforcer for the check.
	CoolDownEnforcer coolDownEnforcer

	// StrategyRunner is the strategy runner for the check.
	StrategyRunner strategy.Runner

	// MetricsGetter is the metrics getter for the check.
	MetricsGetter apm.Looker
}

// checkHandler evaluates one of the checks of a policy.
type checkHandler struct {
	log              hclog.Logger
	coolDownEnforcer coolDownEnforcer
	strategyRunner   strategy.Runner
	metricsGetter    apm.Looker
	check            *sdk.ScalingPolicyCheck
	policy           *sdk.ScalingPolicy
}

// newCheckHandler returns a new checkHandler instance.
func newCheckHandler(config *CheckHandlerConfig, c *sdk.ScalingPolicyCheck) *checkHandler {
	return &checkHandler{
		log: config.Log.Named("check_handler").With(
			"check", c.Name,
			"source", c.Source,
			"strategy", c.Strategy.Name,
		),
		check:            c,
		coolDownEnforcer: config.CoolDownEnforcer,
		strategyRunner:   config.StrategyRunner,
		metricsGetter:    config.MetricsGetter,
	}
}

// start begins the execution of the check handler.
func (h *checkHandler) runChecks(ctx context.Context, currentStatus *sdk.TargetStatus) (*sdk.ScalingAction, error) {
	h.log.Debug("received policy check for evaluation")

	checkEval := &sdk.ScalingCheckEvaluation{
		Check: h.check,
		Action: &sdk.ScalingAction{
			Meta: map[string]interface{}{
				"nomad_policy_id": h.policy.ID,
			},
		},
	}

	// Ensure the Action is canonicalized so we don't need to perform this
	// again.
	checkEval.Action.Canonicalize()

	var err error

	// Query check's APM.
	// Wrap call in a goroutine so we can listen for ctx as well.
	apmQueryDoneCh := make(chan interface{})
	go func() {
		defer close(apmQueryDoneCh)
		checkEval.Metrics, err = h.runAPMQuery()
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-apmQueryDoneCh:
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query source: %v", err)
	}

	if checkEval.Metrics != nil {
		// Make sure metrics are sorted consistently.
		sort.Sort(checkEval.Metrics)

		if len(checkEval.Metrics) == 0 {
			h.log.Warn("no metrics available")
			return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
		}

		if h.log.IsTrace() {
			for _, m := range checkEval.Metrics {
				h.log.Trace("metric result", "ts", m.Timestamp, "value", m.Value)
			}
		}
	} else {
		checkEval.Metrics = sdk.TimestampedMetrics{}
	}

	h.log.Debug("calculating new count", "count", currentStatus.Count)

	runResp, err := h.runStrategyRun(currentStatus.Count, checkEval)
	if err != nil {
		return nil, fmt.Errorf("failed to execute strategy: %v", err)
	}

	if runResp == nil {
		return nil, nil
	}

	checkEval = runResp

	if checkEval.Action.Direction == sdk.ScaleDirectionNone {
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
			checkEval.Action = minMaxAction
		} else {
			h.log.Debug("nothing to do")
			return &sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
		}
	}

	// Canonicalize action so plugins don't have to.
	checkEval.Action.Canonicalize()

	// Make sure new count value is within [min, max] limits
	checkEval.Action.CapCount(h.policy.Min, h.policy.Max)

	return checkEval.Action, nil
}

// runAPMQuery wraps the apm.Query call to provide operational functionality.
func (h *checkHandler) runAPMQuery() (sdk.TimestampedMetrics, error) {
	h.log.Debug("querying source", "query", h.check.Query, "source", h.check.Source)

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: h.check.Source}, {Name: "policy_id", Value: h.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "apm", "query", "invoke_ms"}, time.Now(), labels)

	// Calculate query range from the query window defined in the check.
	to := time.Now().Add(-h.check.QueryWindowOffset)
	from := to.Add(-h.check.QueryWindow)
	r := sdk.TimeRange{From: from, To: to}

	return h.metricsGetter.Query(h.check.Query, r)
}

// runStrategyRun wraps the strategy.Run call to provide operational functionality.
func (h *checkHandler) runStrategyRun(count int64, checkEval *sdk.ScalingCheckEvaluation) (*sdk.ScalingCheckEvaluation, error) {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: h.check.Strategy.Name},
		{Name: "policy_id", Value: h.policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "strategy", "run", "invoke_ms"}, time.Now(), labels)

	return h.strategyRunner.Run(checkEval, count)
}

type checkResult struct {
	action  *sdk.ScalingAction
	handler *checkHandler
}

func (c checkResult) preempt(other checkResult) checkResult {
	winner := sdk.PreemptScalingAction(c.action, other.action)
	if winner == c.action {
		return c
	}
	return other
}
