package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	targetvalue "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/target-value/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the TargetValue Strategy plugin.
func factory(log hclog.Logger) interface{} {
	return targetvalue.NewTargetValuePlugin(log)
}
