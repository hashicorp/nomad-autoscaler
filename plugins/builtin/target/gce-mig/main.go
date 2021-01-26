package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/gce-mig/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Google Cloud Engine MIG plugin.
func factory(log hclog.Logger) interface{} {
	return plugin.NewGCEMIGPlugin(log)
}
