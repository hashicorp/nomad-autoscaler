package plugin

import (
	"fmt"
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
	pluginName = "fixed-value"

	// These are the keys read from the RunRequest.Config map.
	runConfigKeyFixed    = "fixed"

	RADIX_10 = 10

)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewFixedValuePlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}
)

// Assert that StrategyPlugin meets the strategy.Strategy interface.
var _ strategy.Strategy = (*StrategyPlugin)(nil)

// StrategyPlugin is the FixedValue implementation of the strategy.Strategy
// interface.
type StrategyPlugin struct {
	config map[string]string
	logger hclog.Logger
}

// NewFixedValuePlugin returns the FixedValue implementation of the
// strategy.Strategy interface.
func NewFixedValuePlugin(log hclog.Logger) strategy.Strategy {
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

	// Read and parse target value from req.Config.
	t := eval.Check.Strategy.Config[runConfigKeyFixed]
	if t == "" {
		return nil, fmt.Errorf("missing required field `fixed`")
	}

	fixed, err := strconv.ParseInt(t, RADIX_10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `fixed`: %v (%T)", t, t)
	}

	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, fixed)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	// Log at trace level the details of the strategy calculation. This is
	// helpful in ultra-debugging situations when there is a need to understand
	// all the calculations made.
	s.logger.Trace("calculated scaling strategy results",
		"check_name", eval.Check.Name, "current_count", count, "new_count", fixed,
		"direction", eval.Action.Direction)

	eval.Action.Count = fixed
	eval.Action.Reason = fmt.Sprintf("scaling %s because fixed value is %d", eval.Action.Direction, fixed)

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all. It takes into account the current task group count in
// order to correctly account for 0 counts.
//
// The input factor value is padded by e, such that no action will be taken if
// factor is within [1-e; 1+e].
func (s *StrategyPlugin) calculateDirection(count, fixed int64) sdk.ScaleDirection {
	if count == fixed {
		return sdk.ScaleDirectionNone
	} else if count < fixed {
		return sdk.ScaleDirectionUp
	}
	return sdk.ScaleDirectionDown
}
