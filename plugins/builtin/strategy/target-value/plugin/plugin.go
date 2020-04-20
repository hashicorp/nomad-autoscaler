package plugin

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
)

const (
	// pluginName is the unique name of the this plugin amongst strategy
	// plugins.
	pluginName = "target-value"
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
func (s *StrategyPlugin) Run(req strategy.RunRequest) (strategy.RunResponse, error) {
	resp := strategy.RunResponse{Actions: []strategy.Action{}}

	t := req.Config["target"]
	if t == "" {
		return resp, fmt.Errorf("missing required field `target`")
	}

	target, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return resp, fmt.Errorf("invalid value for `target`: %v (%T)", t, t)
	}

	var factor float64

	// Handle cases where the specified target is 0. A potential use case here
	// is targeting a CI build queue to be 0. Adding in build agents when the
	// queue has greater than 0 items in it.
	switch target {
	case 0:
		factor = req.Metric
	default:
		factor = req.Metric / target
	}

	// Identify the direction of scaling, if any.
	direction := s.calculateDirection(req.Count, factor)
	if direction == "" {
		return resp, nil
	}

	var newCount int64

	// Handle cases were users wish to scale from 0. If the current count is 0,
	// then just use the factor as the new count to target. Otherwise use our
	// standard calculation.
	switch req.Count {
	case 0:
		newCount = int64(math.Ceil(factor))
	default:
		newCount = int64(math.Ceil(float64(req.Count) * factor))
	}

	// Log at trace level the details of the strategy calculation. This is
	// helpful in ultra-debugging situations when there is a need to understand
	// all the calculations made.
	s.logger.Trace("calculated scaling strategy results",
		"policy_id", req.PolicyID, "current_count", req.Count, "new_count", newCount,
		"metric_value", req.Metric, "factor", factor, "direction", direction)

	// If the calculated newCount is the same as the current count, we do not
	// need to scale so return an empty response.
	if newCount == req.Count {
		return resp, nil
	}

	action := strategy.Action{
		Count:  &newCount,
		Reason: fmt.Sprintf("scaling %s because factor is %f", direction, factor),
	}
	resp.Actions = append(resp.Actions, action)
	return resp, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all. It takes into account the current task group count in
// order to correctly account for 0 counts.
//
// TODO(jrasell) the direction should probably be a type, rather than a plain
// 	string so we don't have to return an empty string for no direction.
func (s *StrategyPlugin) calculateDirection(count int64, factor float64) string {
	switch count {
	case 0:
		if factor > 0 {
			return "up"
		} else {
			return "down"
		}
	default:
		if factor < 1 {
			return "down"
		} else if factor > 1 {
			return "up"
		} else {
			return ""
		}
	}
}
