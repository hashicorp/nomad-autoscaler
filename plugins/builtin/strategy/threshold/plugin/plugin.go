package plugin

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// pluginName is the unique name of the this plugin amongst strategy
	// plugins.
	pluginName = "threshold"

	// These are the keys read from the RunRequest.Config map.
	runConfigKeyUpperBound          = "upper_bound"
	runConfigKeyLowerBound          = "lower_bound"
	runConfigKeyDelta               = "delta"
	runConfigKeyPercentage          = "percentage"
	runConfigKeyValue               = "value"
	runConfigKeyWithinBoundsTrigger = "within_bounds_trigger"

	// defaultWithinBoundsTrigger is the default value for the
	// within_bounds_trigger check run config.
	defaultWithinBoundsTrigger = 5
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewThresholdPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}
)

// thresholdPluginRunConfig are the parsed values for a threshold plugin run.
type thresholdPluginRunConfig struct {
	upperBound          float64
	lowerBound          float64
	actionType          string
	actionValue         float64
	withinboundsTrigger int
}

// Assert that StrategyPlugin meets the strategy.Strategy interface.
var _ strategy.Strategy = (*StrategyPlugin)(nil)

// StrategyPlugin is the Threshold implementation of the strategy.Strategy
// interface.
type StrategyPlugin struct {
	logger hclog.Logger
}

// NewTargetValuePlugin returns the Threshold implementation of the
// strategy.Strategy interface.
func NewThresholdPlugin(log hclog.Logger) strategy.Strategy {
	return &StrategyPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (s *StrategyPlugin) SetConfig(_ map[string]string) error {
	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (s *StrategyPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Run satisfies the Run function on the strategy.Strategy interface.
func (s *StrategyPlugin) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	if len(eval.Metrics) == 0 {
		s.logger.Trace("no metrics available")
		return nil, nil
	}

	// Parse check config.
	config, err := parseConfig(eval.Check.Strategy.Config)
	if err != nil {
		return nil, err
	}

	logger := s.logger.With("check_name", eval.Check.Name, "current_count", count,
		"lower_bound", config.lowerBound, "upper_bound", config.upperBound,
		"actionType", config.actionType)

	// Check if we have enough data points within bounds.
	if !withinBounds(logger, eval.Metrics, config) {
		logger.Trace("not enough data points within bounds")
		return nil, nil
	}

	// Calculate new count.
	logger.Trace("calculating new count")

	var newCount int64
	switch config.actionType {
	case runConfigKeyDelta:
		newCount = runDelta(count, config.actionValue)
	case runConfigKeyPercentage:
		newCount = runPercentage(count, config.actionValue)
	case runConfigKeyValue:
		newCount = runValue(config.actionValue)
	}

	// Identify the direction of scaling, and exit early if none.
	eval.Action.Direction = calculateDirection(count, newCount)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	logger.Trace("calculated scaling strategy results",
		"new_count", newCount, "direction", eval.Action.Direction)

	eval.Action.Count = newCount
	eval.Action.Reason = fmt.Sprintf("scaling %s because metric is within bounds", eval.Action.Direction)

	return eval, nil
}

// parseConfig parses and validates the policy check config.
func parseConfig(config map[string]string) (*thresholdPluginRunConfig, error) {
	c := &thresholdPluginRunConfig{}

	// Read and parse threshold bounds from check config.
	upperStr := config[runConfigKeyUpperBound]
	lowerStr := config[runConfigKeyLowerBound]
	if upperStr == "" && lowerStr == "" {
		return nil, fmt.Errorf("missing required field, must have either %q or %q", runConfigKeyLowerBound, runConfigKeyUpperBound)
	}

	upper, err := parseBound(runConfigKeyUpperBound, upperStr)
	if err != nil {
		return nil, err
	}
	c.upperBound = upper

	lower, err := parseBound(runConfigKeyLowerBound, lowerStr)
	if err != nil {
		return nil, err
	}
	c.lowerBound = lower

	// Read and parse within bounds trigger from check config.
	triggerStr := config[runConfigKeyWithinBoundsTrigger]

	trigger := defaultWithinBoundsTrigger
	if triggerStr != "" {
		t, err := strconv.ParseInt(triggerStr, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid value for %q: %v (%T)", runConfigKeyWithinBoundsTrigger, triggerStr, triggerStr)
		}
		trigger = int(t)
	}
	c.withinboundsTrigger = trigger

	// Read and validate action type from check config.
	deltaStr := config[runConfigKeyDelta]
	percentageStr := config[runConfigKeyPercentage]
	valueStr := config[runConfigKeyValue]

	nonEmpty := 0
	for _, s := range []string{deltaStr, percentageStr, valueStr} {
		if s != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return nil, fmt.Errorf("missing required field, must have either %q, %q or %q", runConfigKeyDelta, runConfigKeyPercentage, runConfigKeyValue)
	}
	if nonEmpty != 1 {
		return nil, fmt.Errorf("only one of %q, %q or %q must be provided", runConfigKeyDelta, runConfigKeyPercentage, runConfigKeyValue)
	}

	if deltaStr != "" {
		// Delta must be an interger.
		d, err := strconv.ParseInt(deltaStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q value %v is not an interger", runConfigKeyDelta, deltaStr)
		}

		c.actionType = runConfigKeyDelta
		c.actionValue = float64(d)
	}

	if percentageStr != "" {
		// Percentage must be a float.
		p, err := strconv.ParseFloat(percentageStr, 64)
		if err != nil {
			return nil, fmt.Errorf("%q value %v is not a number", runConfigKeyPercentage, percentageStr)
		}

		c.actionType = runConfigKeyPercentage
		c.actionValue = p
	}

	if valueStr != "" {
		// Value must be a positive interger.
		v, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q value %v is not an interger", runConfigKeyValue, valueStr)
		}
		if v < 0 {
			return nil, fmt.Errorf("%q value %v is negative", runConfigKeyValue, valueStr)
		}

		c.actionType = runConfigKeyValue
		c.actionValue = float64(v)
	}

	return c, nil
}

// parseBound parses and validates the value for a bound.
func parseBound(bound string, input string) (float64, error) {
	var defaultValue float64

	switch bound {
	case runConfigKeyLowerBound:
		defaultValue = -math.MaxFloat64
	case runConfigKeyUpperBound:
		defaultValue = math.MaxFloat64
	}

	if input == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %q: %v (%T)", bound, input, input)
	}

	return value, nil
}

// withinBounds returns true if the metric result is considered within bounds.
func withinBounds(logger hclog.Logger, metrics sdk.TimestampedMetrics, config *thresholdPluginRunConfig) bool {
	logger.Trace("checking how many data points are within bounds")

	withinBoundsCounter := 0
	for _, metric := range metrics {
		if metric.Value >= config.lowerBound && metric.Value < config.upperBound {
			withinBoundsCounter++
		}
	}

	logger.Trace(fmt.Sprintf("found %d data points within bounds", withinBoundsCounter))
	return withinBoundsCounter >= config.withinboundsTrigger
}

// runDelta returns the next count for a delta check.
func runDelta(count int64, d float64) int64 {
	return count + int64(d)
}

// runPercentage returns the next count for a percentage check.
func runPercentage(count int64, pct float64) int64 {
	newCount := float64(count) * (1 + pct/100)
	return int64(math.Ceil(newCount))
}

// runValue returns the next count for a value check.
func runValue(v float64) int64 {
	return int64(v)
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all.
func calculateDirection(currentCount, newCount int64) sdk.ScaleDirection {
	switch {
	case newCount > currentCount:
		return sdk.ScaleDirectionUp
	case newCount < currentCount:
		return sdk.ScaleDirectionDown
	default:
		return sdk.ScaleDirectionNone
	}
}
