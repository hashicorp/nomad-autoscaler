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
	pluginName = "target-value"

	// These are the keys read from the RunRequest.Config map.
	runConfigKeyTarget    = "target"
	runConfigKeyThreshold = "threshold"

	// defaultThreshold controls how significant is a change in the input
	// metric value.
	defaultThreshold = "0.01"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewTargetValuePlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeStrategy,
	}
)

// Assert that StrategyPlugin meets the strategy.Strategy interface.
var _ strategy.Strategy = (*StrategyPlugin)(nil)

// StrategyPlugin is the TargetValue implementation of the strategy.Strategy
// interface.
type StrategyPlugin struct {
	config map[string]string
	logger hclog.Logger
}

// NewTargetValuePlugin returns the TargetValue implementation of the
// strategy.Strategy interface.
func NewTargetValuePlugin(log hclog.Logger) strategy.Strategy {
	return &StrategyPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Plugin interface.
func (s *StrategyPlugin) SetConfig(config map[string]string) error {
	s.config = config
	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Plugin interface.
func (s *StrategyPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Run satisfies the Run function on the strategy.Strategy interface.
func (s *StrategyPlugin) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {

	// Read and parse target value from req.Config.
	t := eval.Check.Strategy.Config[runConfigKeyTarget]
	if t == "" {
		return nil, fmt.Errorf("missing required field `target`")
	}

	target, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `target`: %v (%T)", t, t)
	}

	// Read and parse threshold value from req.Config.
	th := eval.Check.Strategy.Config[runConfigKeyThreshold]
	if th == "" {
		th = defaultThreshold
	}

	threshold, err := strconv.ParseFloat(th, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `threshold`: %v (%T)", th, th)
	}

	var factor float64

	// Handle cases where the specified target is 0. A potential use case here
	// is targeting a CI build queue to be 0. Adding in build agents when the
	// queue has greater than 0 items in it.
	switch target {
	case 0:
		factor = eval.Metric
	default:
		factor = eval.Metric / target
	}

	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, factor, threshold)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	var newCount int64

	// Handle cases were users wish to scale from 0. If the current count is 0,
	// then just use the factor as the new count to target. Otherwise use our
	// standard calculation.
	switch count {
	case 0:
		newCount = int64(math.Ceil(factor))
	default:
		newCount = int64(math.Ceil(float64(count) * factor))
	}

	// Log at trace level the details of the strategy calculation. This is
	// helpful in ultra-debugging situations when there is a need to understand
	// all the calculations made.
	s.logger.Trace("calculated scaling strategy results",
		"check_name", eval.Check.Name, "current_count", count, "new_count", newCount,
		"metric_value", eval.Metric, "factor", factor, "direction", eval.Action.Direction)

	// If the calculated newCount is the same as the current count, we do not
	// need to scale so return an empty response.
	if newCount == count {
		eval.Action.Direction = sdk.ScaleDirectionNone
		return eval, nil
	}

	eval.Action.Count = newCount
	eval.Action.Reason = fmt.Sprintf("scaling %s because factor is %f", eval.Action.Direction, factor)

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all. It takes into account the current task group count in
// order to correctly account for 0 counts.
//
// The input factor value is padded by e, such that no action will be taken if
// factor is within [1-e; 1+e].
func (s *StrategyPlugin) calculateDirection(count int64, factor, e float64) sdk.ScaleDirection {
	switch count {
	case 0:
		if factor > 0 {
			return sdk.ScaleDirectionUp
		}
		return sdk.ScaleDirectionNone
	default:
		if factor < (1 - e) {
			return sdk.ScaleDirectionDown
		} else if factor > (1 + e) {
			return sdk.ScaleDirectionUp
		} else {
			return sdk.ScaleDirectionNone
		}
	}
}
