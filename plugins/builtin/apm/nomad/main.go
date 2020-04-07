package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	nomadapm "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/nomad/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins: map[string]plugin.Plugin{
			plugins.PluginTypeAPM: &apm.Plugin{Impl: &nomadapm.APMPlugin{}},
		},
	})
}
