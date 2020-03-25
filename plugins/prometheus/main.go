package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/apm"
	prometheus "github.com/hashicorp/nomad-autoscaler/plugins/prometheus/apm"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	p := &prometheus.APM{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"apm": &apm.Plugin{Impl: p},
		},
	})
}
