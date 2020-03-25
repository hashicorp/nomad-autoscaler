package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/apm"
	nomadapm "github.com/hashicorp/nomad-autoscaler/plugins/nomad/apm"
	nomadtarget "github.com/hashicorp/nomad-autoscaler/plugins/nomad/target"
	"github.com/hashicorp/nomad-autoscaler/target"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"apm":    &apm.Plugin{Impl: &nomadapm.MetricsAPM{}},
			"target": &target.Plugin{Impl: &nomadtarget.NomadGroupCount{}},
		},
	})
}
