// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	pluginName = "pass-through"

	// These are the keys read from the RunRequest.Config map.
	runConfigKeyMaxScaleUp   = "max_scale_up"
	runConfigKeyMaxScaleDown = "max_scale_down"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewPassThroughPlugin(l) },
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

// NewPassThroughPlugin returns the Pass Through implementation of the
// strategy.Strategy interface.
func NewPassThroughPlugin(log hclog.Logger) strategy.Strategy {
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
	// Read and parse max_scale_up from req.Config.
	var maxScaleUp *int64
	maxScaleUpStr := eval.Check.Strategy.Config[runConfigKeyMaxScaleUp]
	if maxScaleUpStr != "" {
		msu, err := strconv.ParseInt(maxScaleUpStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for `max_scale_up`: %v (%T)", maxScaleUpStr, maxScaleUpStr)
		}
		maxScaleUp = &msu
	} else {
		maxScaleUpStr = "+Inf"
	}

	// Read and parse max_scale_down from req.Config.
	var maxScaleDown *int64
	maxScaleDownStr := eval.Check.Strategy.Config[runConfigKeyMaxScaleDown]
	if maxScaleDownStr != "" {
		msd, err := strconv.ParseInt(maxScaleDownStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for `max_scale_down`: %v (%T)", maxScaleDownStr, maxScaleDownStr)
		}
		maxScaleDown = &msd
	} else {
		maxScaleDownStr = "-Inf"
	}

	if len(eval.Metrics) == 0 {
		return nil, nil
	}

	// Use only the latest value for now.
	metric := eval.Metrics[len(eval.Metrics)-1]

	// Identify the direction of scaling, if any.
	eval.Action.Direction = s.calculateDirection(count, metric.Value)
	if eval.Action.Direction == sdk.ScaleDirectionNone {
		return eval, nil
	}

	newCount := int64(metric.Value)

	switch eval.Action.Direction {
	case sdk.ScaleDirectionUp:
		if maxScaleUp != nil && newCount > count+*maxScaleUp {
			newCount = count + *maxScaleUp
		}
	case sdk.ScaleDirectionDown:
		if maxScaleDown != nil && newCount < count-*maxScaleDown {
			newCount = count - *maxScaleDown
		}
	}

	// Log at trace level the details of the strategy calculation.
	s.logger.Trace("calculated scaling strategy results",
		"check_name", eval.Check.Name, "current_count", count, "new_count", newCount,
		"metric_value", metric.Value, "metric_time", metric.Timestamp,
		"direction", eval.Action.Direction, "max_scale_up", maxScaleUpStr, "max_scale_down", maxScaleDownStr)

	eval.Action.Count = newCount
	eval.Action.Reason = fmt.Sprintf("scaling %s because metric is %d", eval.Action.Direction, int64(metric.Value))

	return eval, nil
}

// calculateDirection is used to calculate the direction of scaling that should
// occur, if any at all.
func (s *StrategyPlugin) calculateDirection(count int64, metric float64) sdk.ScaleDirection {
	if metric > float64(count) {
		return sdk.ScaleDirectionUp
	} else if metric < float64(count) {
		return sdk.ScaleDirectionDown
	} else {
		return sdk.ScaleDirectionNone
	}
}
