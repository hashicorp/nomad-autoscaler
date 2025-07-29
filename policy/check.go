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

type CheckHandlerConfig struct {
	// Log is the logger to use for logging.
	Log hclog.Logger

	// StrategyRunner is the strategy runner for the check.
	StrategyRunner strategy.Runner

	// MetricsGetter is the metrics getter for the check.
	MetricsGetter apm.Looker

	Policy *sdk.ScalingPolicy
}

// checkHandler evaluates one of the checks of a policy.
type checkHandler struct {
	log            hclog.Logger
	strategyRunner strategy.Runner
	metricsGetter  apm.Looker
	check          *sdk.ScalingPolicyCheck
	policy         *sdk.ScalingPolicy
}

// newCheckHandler returns a new checkHandler instance.
func newCheckHandler(config *CheckHandlerConfig, c *sdk.ScalingPolicyCheck) *checkHandler {
	return &checkHandler{
		log: config.Log.Named("check_handler").With(
			"check", c.Name,
			"source", c.Source,
			"strategy", c.Strategy.Name,
		),
		check:          c,
		strategyRunner: config.StrategyRunner,
		metricsGetter:  config.MetricsGetter,
		policy:         config.Policy,
	}
}

// start begins the execution of the check handler.
func (ch *checkHandler) getNewCountFromMetrics(ctx context.Context, currentCount int64, metrics sdk.TimestampedMetrics) (sdk.ScalingAction, error) {
	a, err := ch.runStrategy(ctx, currentCount, metrics)
	if err != nil {
		ch.log.Warn("failed to run check", "check", ch.check.Name,
			"on_error", ch.check.OnError, "on_check_error",
			ch.policy.OnCheckError, "error", err)

		// Define how to handle error.
		// Use check behaviour if set or fail iff the policy is set to fail.
		switch ch.check.OnError {
		case sdk.ScalingPolicyOnErrorIgnore:
			return sdk.ScalingAction{}, nil
		case sdk.ScalingPolicyOnErrorFail:
			return sdk.ScalingAction{}, err
		default:
			if ch.policy.OnCheckError == sdk.ScalingPolicyOnErrorFail {
				return sdk.ScalingAction{}, err
			}
		}

		return sdk.ScalingAction{}, nil
	}

	return a, nil
}

// start begins the execution of the check handler.
func (ch *checkHandler) runStrategy(ctx context.Context, currentCount int64, metrics sdk.TimestampedMetrics) (sdk.ScalingAction, error) {
	ch.log.Debug("received policy check for evaluation")

	checkEval := &sdk.ScalingCheckEvaluation{
		Check: ch.check,
		Action: &sdk.ScalingAction{
			Meta: map[string]interface{}{
				"nomad_policy_id": ch.policy.ID,
			},
		},
	}

	ch.log.Debug("calculating new count", "count", currentCount)

	runResp, err := ch.runStrategyRun(currentCount, checkEval)
	if err != nil {
		return *checkEval.Action, fmt.Errorf("failed to execute strategy: %v", err)
	}

	if runResp == nil {
		return *checkEval.Action, nil
	}

	checkEval = runResp

	if checkEval.Action.Direction == sdk.ScaleDirectionNone {
		// Make sure we are currently within [min, max] limits even if there's
		// no action to execute
		var minMaxAction *sdk.ScalingAction

		if currentCount < ch.policy.Min {
			minMaxAction = &sdk.ScalingAction{
				Count:     ch.policy.Min,
				Direction: sdk.ScaleDirectionUp,
				Reason:    fmt.Sprintf("current count (%d) below limit (%d)", currentCount, ch.policy.Min),
			}
		} else if currentCount > ch.policy.Max {
			minMaxAction = &sdk.ScalingAction{
				Count:     ch.policy.Max,
				Direction: sdk.ScaleDirectionDown,
				Reason:    fmt.Sprintf("current count (%d) above limit (%d)", currentCount, ch.policy.Max),
			}
		}

		if minMaxAction != nil {
			checkEval.Action = minMaxAction
		} else {
			ch.log.Debug("nothing to do")
			return sdk.ScalingAction{Direction: sdk.ScaleDirectionNone}, nil
		}
	}

	// Canonicalize action so plugins don't have to.
	checkEval.Action.Canonicalize()

	// Make sure new count value is within [min, max] limits
	checkEval.Action.CapCount(ch.policy.Min, ch.policy.Max)

	return *checkEval.Action, nil
}

// runStrategyRun wraps the strategy.Run call to provide operational functionality.
func (ch *checkHandler) runStrategyRun(count int64, checkEval *sdk.ScalingCheckEvaluation) (*sdk.ScalingCheckEvaluation, error) {

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: ch.check.Strategy.Name},
		{Name: "policy_id", Value: ch.policy.ID},
	}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "strategy", "run", "invoke_ms"}, time.Now(), labels)

	return ch.strategyRunner.Run(checkEval, count)
}

// runAPMQuery wraps the apm.Query call to provide operational functionality.
func (ch *checkHandler) runAPMQuery(ctx context.Context) (sdk.TimestampedMetrics, error) {
	ch.log.Debug("querying source", "query", ch.check.Query, "source", ch.check.Source)

	// Trigger a metric measure to track latency of the call.	labels := []metrics.Label{{Name: "plugin_name", Value: ch.check.Source}, {Name: "policy_id", Value: ch.policy.ID}}
	labels := []metrics.Label{{Name: "plugin_name", Value: ch.check.Source}, {Name: "policy_id", Value: ch.policy.ID}}

	defer metrics.MeasureSinceWithLabels([]string{"plugin", "apm", "query", "invoke_ms"}, time.Now(), labels)

	// Calculate query range from the query window defined in the ch.check.
	to := time.Now().Add(-ch.check.QueryWindowOffset)
	from := to.Add(-ch.check.QueryWindow)
	r := sdk.TimeRange{From: from, To: to}

	var err error
	metrics := sdk.TimestampedMetrics{}

	// Query check's APM.
	// Wrap call in a goroutine so we can listen for ctx as well.
	apmQueryDoneCh := make(chan interface{})
	go func() {
		defer close(apmQueryDoneCh)
		metrics, err = ch.metricsGetter.Query(ch.check.Query, r)
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-apmQueryDoneCh:
	}

	if err != nil {
		return sdk.TimestampedMetrics{}, fmt.Errorf("failed to query source: %v", err)
	}

	if len(metrics) == 0 {
		return sdk.TimestampedMetrics{}, errNoMetrics
	}

	// Make sure metrics are sorted consistently.
	sort.Sort(metrics)

	return metrics, nil
}

type checkResult struct {
	action  *sdk.ScalingAction
	handler *checkHandler
	group   string
}

func (c checkResult) preempt(other checkResult) checkResult {
	winner := sdk.PreemptScalingAction(c.action, other.action)
	if winner == c.action {
		return c
	}
	return other
}
