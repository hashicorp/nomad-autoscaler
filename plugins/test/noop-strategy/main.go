package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "noop-strategy"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeStrategy,
	}
)

var _ strategy.Strategy = (*Noop)(nil)

type Noop struct {
	logger hclog.Logger
}

func (n *Noop) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	n.logger.Debug("run")
	eval.Action = nil
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
