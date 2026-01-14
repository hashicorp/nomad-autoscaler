// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"fmt"
	"sort"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

var nowFunc = time.Now

type CheckRunnerConfig struct {
	// Log is the logger to use for logging.
	Log hclog.Logger

	// StrategyRunner is the strategy runner for the check.
	StrategyRunner strategy.Runner

	// MetricsGetter is the metrics getter for the check.
	MetricsGetter apm.Looker

	Policy *sdk.ScalingPolicy
}

// checkHandler evaluates one of the checks of a policy.
type checkRunner struct {
	log            hclog.Logger
	strategyRunner strategy.Runner
	metricsGetter  apm.Looker
	check          *sdk.ScalingPolicyCheck
	policy         *sdk.ScalingPolicy
}

// NewCheckHandler returns a new checkHandler instance.
func newCheckRunner(config *CheckRunnerConfig, c *sdk.ScalingPolicyCheck) *checkRunner {
	return &checkRunner{
		log:            config.Log,
		check:          c,
		strategyRunner: config.StrategyRunner,
		metricsGetter:  config.MetricsGetter,
		policy:         config.Policy,
	}
}

// getNewCountFromStrategy begins the execution of the checks and returns
// and action containing the instance count after applying the strategy to the
// metrics.
func (ch *checkRunner) getNewCountFromStrategy(ctx context.Context, currentCount int64,
	metrics sdk.TimestampedMetrics) (sdk.ScalingAction, error) {
	ch.log.Debug("calculating new count", "current count", currentCount)

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
	} else {
		ch.log.Debug("strategy returned count", "strategy", ch.check.Strategy.Name, "check", ch.check.Name, "count", a.Count, "reason", a.Reason)
	}

	return a, nil
}

func (ch *checkRunner) runStrategy(ctx context.Context, currentCount int64, ms sdk.TimestampedMetrics) (sdk.ScalingAction, error) {
	checkEval := &sdk.ScalingCheckEvaluation{
		Check: ch.check,
		Action: &sdk.ScalingAction{
			Meta: map[string]interface{}{
				"nomad_policy_id": ch.policy.ID,
			},
		},
		Metrics: ms,
	}

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{
		{Name: "plugin_name", Value: ch.check.Strategy.Name},
		{Name: "policy_id", Value: ch.policy.ID},
	}

	defer metrics.MeasureSinceWithLabels([]string{"plugin", "strategy", "run", "invoke_ms"}, time.Now(), labels)

	var runResp *sdk.ScalingCheckEvaluation
	var err error

	ch.log.Debug("running strategy", "strategy", ch.check.Strategy.Name, "check", ch.check.Name)

	// Wrap call in a goroutine so we can listen for ctx as well.
	strategyRunDoneCh := make(chan interface{})
	go func() {
		defer close(strategyRunDoneCh)
		runResp, err = ch.strategyRunner.Run(checkEval, currentCount)
	}()

	select {
	case <-ctx.Done():
		return sdk.ScalingAction{}, nil
	case <-strategyRunDoneCh:
	}

	if err != nil {
		return sdk.ScalingAction{}, fmt.Errorf("failed to execute strategy: %w", err)
	}

	if runResp == nil {
		return sdk.ScalingAction{}, nil
	}

	return *runResp.Action, nil
}

// queryMetrics wraps the apm.Query call to provide operational functionality.
func (ch *checkRunner) queryMetrics(ctx context.Context) (sdk.TimestampedMetrics, error) {
	ch.log.Debug("querying source", "query", ch.check.Query, "source", ch.check.Source)

	// Trigger a metric measure to track latency of the call.
	labels := []metrics.Label{{Name: "plugin_name", Value: ch.check.Source},
		{Name: "policy_id", Value: ch.policy.ID}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "apm", "query", "invoke_ms"},
		time.Now(), labels)

	// Calculate query range from the query window defined in the ch.check.
	to := nowFunc().Add(-ch.check.QueryWindowOffset)
	from := to.Add(-ch.check.QueryWindow)
	r := sdk.TimeRange{From: from, To: to}

	var err error
	ms := sdk.TimestampedMetrics{}

	// Query check's APM.
	// Wrap call in a goroutine so we can listen for ctx as well.
	apmQueryDoneCh := make(chan interface{})
	go func() {
		defer close(apmQueryDoneCh)
		ms, err = ch.metricsGetter.Query(ch.check.Query, r)
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-apmQueryDoneCh:
	}

	if err != nil {
		return sdk.TimestampedMetrics{}, fmt.Errorf("failed to query source: %w", err)
	}

	if len(ms) == 0 {
		return sdk.TimestampedMetrics{}, errNoMetrics
	}

	// Make sure metrics are sorted consistently.
	sort.Sort(ms)

	return ms, nil
}

func (ch *checkRunner) group() string {
	return ch.check.Group
}

func (ch *checkRunner) runCheckAndCapCount(ctx context.Context, currentCount int64) (sdk.ScalingAction, error) {
	ch.log.Debug("received policy check for evaluation")

	metrics, err := ch.queryMetrics(ctx)
	if err != nil {
		return sdk.ScalingAction{}, fmt.Errorf("failed to query source: %v", err)
	}

	action, err := ch.getNewCountFromStrategy(ctx, currentCount, metrics)
	if err != nil {
		return sdk.ScalingAction{}, fmt.Errorf("failed get count from metrics: %v", err)

	}

	action.CapCount(ch.policy.Min, ch.policy.Max)

	return action, nil
}
