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
	runConfigKeyValue = "value"
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

	// Read and parse fixed value from req.Config.
	t := eval.Check.Strategy.Config[runConfigKeyValue]
	if t == "" {
		return nil, fmt.Errorf("missing required field `%s`", runConfigKeyValue)
	}

	value, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `%s`: %v (%T)", runConfigKeyValue, t, t)
	}

	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, value)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	s.logger.Info("calculated scaling strategy results",
		"check_name", eval.Check.Name, "current_count", count, "new_count", value,
		"direction", eval.Action.Direction)

	eval.Action.Count = value
	eval.Action.Reason = fmt.Sprintf("scaling %s because fixed value is %d", eval.Action.Direction, value)

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all.
func (s *StrategyPlugin) calculateDirection(count, fixed int64) sdk.ScaleDirection {
	if count == fixed {
		return sdk.ScaleDirectionNone
	} else if count < fixed {
		return sdk.ScaleDirectionUp
	}
	return sdk.ScaleDirectionDown
}
