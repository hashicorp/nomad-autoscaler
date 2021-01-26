package main

import (
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "noop-strategy"

	runConfigKeyCount     = "count"
	runConfigKeyReason    = "reason"
	runConfigKeyError     = "error"
	runConfigKeyDirection = "direction"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeStrategy,
	}
)

var _ strategy.Strategy = (*Noop)(nil)

type Noop struct {
	logger hclog.Logger
}

func (n *Noop) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	config := eval.Check.Strategy.Config

	action := &sdk.ScalingAction{
		Reason: config[runConfigKeyReason],
		Error:  config[runConfigKeyError] == "true",
	}

	countStr := config[runConfigKeyCount]
	action.Count, _ = strconv.ParseInt(countStr, 10, 64)

	if config[runConfigKeyDirection] == "up" {
		action.Direction = sdk.ScaleDirectionUp
	} else if config[runConfigKeyDirection] == "down" {
		action.Direction = sdk.ScaleDirectionDown
	}

	n.logger.Debug("run", "action", action)

	eval.Action = action
	return eval, nil
}

func (n *Noop) PluginInfo() (*base.PluginInfo, error) {
	n.logger.Debug("plugin info")
	return pluginInfo, nil
}

func (n *Noop) SetConfig(config map[string]string) error {
	n.logger.Debug("set config", "config", config)
	return nil
}

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &Noop{logger: l}
}
