package main

import (
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/target"
	"github.com/hashicorp/nomad-autoscaler/target/plugins/nomad/nomad"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	p := &nomad.NomadGroupCount{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"target": &target.Plugin{Impl: p},
		},
	})

}
