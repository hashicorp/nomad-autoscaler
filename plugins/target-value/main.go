package main

import (
	"github.com/hashicorp/go-plugin"
	targetvalue "github.com/hashicorp/nomad-autoscaler/plugins/target-value/strategy"
	"github.com/hashicorp/nomad-autoscaler/strategy"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	s := &targetvalue.Strategy{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"strategy": &strategy.Plugin{Impl: s},
		},
	})

}
