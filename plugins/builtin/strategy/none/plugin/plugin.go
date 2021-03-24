package plugin

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// pluginName is the unique name of the this plugin amongst strategy
	// plugins.
	pluginName = "none"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewTargetValuePlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}
)

// Assert that StrategyPlugin meets the strategy.Strategy interface.
var _ strategy.Strategy = (*StrategyPlugin)(nil)

// StrategyPlugin is the None implementation of the strategy.Strategy
// interface.
type StrategyPlugin struct {
	logger hclog.Logger
}

// NewTargetValuePlugin returns the None implementation of the
// strategy.Strategy interface.
func NewTargetValuePlugin(log hclog.Logger) strategy.Strategy {
	return &StrategyPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (s *StrategyPlugin) SetConfig(config map[string]string) error {
	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (s *StrategyPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Run satisfies the Run function on the strategy.Strategy interface.
func (s *StrategyPlugin) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {

	// Use only the latest value for now.
	metric := eval.Metrics[len(eval.Metrics)-1]


	// TODO: compare count > metric --> direction
	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, metric.Value)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}


	eval.Action.Count = int64(metric.Value)
	eval.Action.Reason = fmt.Sprintf("scaling %s because metric is %d", eval.Action.Direction, eval.Action.Count)

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all. It takes into account the current task group count in
// order to correctly account for 0 counts.
//
// The input factor value is padded by e, such that no action will be taken if
// factor is within [1-e; 1+e].
func (s *StrategyPlugin) calculateDirection(count int64, metric float64) sdk.ScaleDirection {
	if metric > float64(count) {
		return sdk.ScaleDirectionUp
	} else if metric < float64(count) {
		return sdk.ScaleDirectionDown
	} else {
		return sdk.ScaleDirectionNone
	}
}
