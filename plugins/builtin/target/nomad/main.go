package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadtarget "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/nomad/plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins: map[string]plugin.Plugin{
			plugins.PluginTypeTarget: &target.Plugin{Impl: &nomadtarget.TargetPlugin{}},
		},
	})
}
