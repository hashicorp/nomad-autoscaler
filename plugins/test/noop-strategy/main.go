package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
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

func (n *Noop) Run(req strategy.RunRequest) (strategy.RunResponse, error) {
	n.logger.Info("Run")
	return strategy.RunResponse{Actions: []strategy.Action{}}, nil
}

func (n *Noop) PluginInfo() (*base.PluginInfo, error) {
	n.logger.Debug("PluginInfo")
	return pluginInfo, nil
}

func (n *Noop) SetConfig(config map[string]string) error {
	n.logger.Debug("SetConfig", "config", config)
	return nil
}

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &Noop{logger: l}
}
