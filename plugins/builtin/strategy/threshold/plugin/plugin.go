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
	runConfigKeyUpperBound    = "upper-bound"
	runConfigKeyLowerBound = "lower-bound"
	runConfigKeyDelta = "delta"
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

// Assert that StrategyPlugin meets the strategy.Strategy interface.
var _ strategy.Strategy = (*StrategyPlugin)(nil)

// StrategyPlugin is the Threshold implementation of the strategy.Strategy
// interface.
type StrategyPlugin struct {
	config map[string]string
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
func (s *StrategyPlugin) SetConfig(config map[string]string) error {
	s.config = config
	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (s *StrategyPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Run satisfies the Run function on the strategy.Strategy interface.
func (s *StrategyPlugin) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {

	// Read and parse threshold bounds from req.Config.
	upperbound := eval.Check.Strategy.Config[runConfigKeyUpperBound]
	lowerbound := eval.Check.Strategy.Config[runConfigKeyLowerBound]
	if upperbound == "" && lowerbound == "" {
		return nil, fmt.Errorf("missing required fields, must have either `lower-bound` or `upper-bound`")
	}

	//var upper, lower float64
	//if upperbound != "" {
		upper, err := strconv.ParseFloat(upperbound, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for `upper-bound`: %v (%T)", upperbound, upperbound)
		}
	//}
	//if lowerbound != "" {
		lower, err := strconv.ParseFloat(lowerbound, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for `lower-bound`: %v (%T)", lowerbound, lowerbound)
		}
	//}

	// Read and parse threshold value from req.Config.
	d := eval.Check.Strategy.Config[runConfigKeyDelta]
	if d == "" {
		return nil, fmt.Errorf("missing required fields `delta`")
	}

	delta, err := strconv.ParseFloat(d, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `delta`: %v (%T)", d, d)
	}

	var factor float64

	// This shouldn't happen, but check it just in case.
	if len(eval.Metrics) == 0 {
		return nil, nil
	}

	// Use only the latest value for now.
	metric := eval.Metrics[len(eval.Metrics)-1]

	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, upper, lower)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	var newCount int64

	// Handle cases were users wish to scale from 0. If the current count is 0,
	// then just use the factor as the new count to target. Otherwise use our
	// standard calculation.
	switch count {
	case 0:
		newCount = int64(math.Ceil(delta))
	default:
		if metric.Value > upper {
			newCount = int64(math.Ceil(float64(count) + delta))
		} else if metric.Value < lower {
			newCount = int64(math.Ceil(float64(count) + delta))
		}
	}

	// Log at trace level the details of the strategy calculation. This is
	// helpful in ultra-debugging situations when there is a need to understand
	// all the calculations made.
	s.logger.Trace("calculated scaling strategy results",
		"check_name", eval.Check.Name, "current_count", count, "new_count", newCount,
		"metric_value", metric.Value, "metric_time", metric.Timestamp, "factor", factor,
		"direction", eval.Action.Direction)

	// If the calculated newCount is the same as the current count, we do not
	// need to scale so return an empty response.
	if newCount == count {
		eval.Action.Direction = sdk.ScaleDirectionNone
		return eval, nil
	}

	eval.Action.Count = newCount
	eval.Action.Reason = fmt.Sprintf("scaling %s because metric passed threshold", eval.Action.Direction)

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all. It takes into account the current task group count in
// order to correctly account for 0 counts.
//
// The input factor value is padded by e, such that no action will be taken if
// factor is within [1-e; 1+e].
func (s *StrategyPlugin) calculateDirection(count int64, upper, lower float64) sdk.ScaleDirection {
	switch count {
	case 0:
		return sdk.ScaleDirectionUp
	default:
		c := float64(count)
		if c < lower {
			return sdk.ScaleDirectionDown
		} else if c > upper {
			return sdk.ScaleDirectionUp
		} else {
			return sdk.ScaleDirectionNone
		}
	}
}
