package main

import (
	"flag"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/apm"
	"github.com/hashicorp/nomad-autoscaler/apm/plugins/nomad-apm/nomad"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

func main() {
	p := &nomad.MetricsAPM{}

	sandbox := flag.Bool("sandbox", false, "")
	query := flag.String("query", "", "")
	flag.Parse()

	if sandbox != nil && *sandbox {
		err := p.SetConfig(map[string]string{"address": "127.0.0.1:4646"})
		if err != nil {
			fmt.Println(err)
			return
		}
		result, err := p.Query(*query)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Result: %f", result)
		return
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"apm": &apm.Plugin{Impl: p},
		},
	})
}
