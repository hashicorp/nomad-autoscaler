package main

import (
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "noop-target"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

var _ target.Target = (*Noop)(nil)

type Noop struct {
	logger hclog.Logger
}

func (n *Noop) Scale(action sdk.ScalingAction, config map[string]string) error {
	n.logger.Debug("received scale action", "count", action.Count, "reason", action.Reason)
	return nil
}

func (n *Noop) Status(config map[string]string) (*sdk.TargetStatus, error) {
	var count int64
	countStr := config["count"]
	if countStr != "" {
		count, _ = strconv.ParseInt(countStr, 10, 64)
	}

	ready := !(config["ready"] == "false")

	n.logger.Debug("received status request", "count", count, "ready", ready)

	return &sdk.TargetStatus{
		Count: count,
		Ready: ready,
	}, nil
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
