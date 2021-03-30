package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	passthru "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/pass-thru/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the PassThru Strategy plugin.
func factory(log hclog.Logger) interface{} {
	return passthru.NewPassThruPlugin(log)
}
