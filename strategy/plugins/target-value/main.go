package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/strategy"
	"github.com/hashicorp/nomad-autoscaler/strategy/plugins/target-value/targetvalue"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	s := &targetvalue.TargetValue{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"strategy": &strategy.Plugin{Impl: s},
		},
	})

}
