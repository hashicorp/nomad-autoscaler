package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	datadog "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/datadog/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Datadog APM plugin.
func factory(log hclog.Logger) interface{} {
	return datadog.NewDatadogPlugin(log)
}
