package main

import (
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	prometheus "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/prometheus/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins: map[string]plugin.Plugin{
			plugins.PluginTypeAPM: &apm.Plugin{Impl: &prometheus.APMPlugin{}},
		},
	})
}
