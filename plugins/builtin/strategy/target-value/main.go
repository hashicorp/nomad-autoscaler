package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	targetvalue "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/target-value/plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins: map[string]plugin.Plugin{
			plugins.PluginTypeStrategy: &strategy.Plugin{Impl: &targetvalue.StrategyPlugin{}},
		},
	})
}
