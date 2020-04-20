package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	prometheus "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/prometheus/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Prometheus APM plugin.
func factory(log hclog.Logger) interface{} {
	return prometheus.NewPrometheusPlugin(log)
}
