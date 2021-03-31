package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	passthrough "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/pass-through/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the PassThrough Strategy plugin.
func factory(log hclog.Logger) interface{} {
	return passthrough.NewPassThroughPlugin(log)
}
