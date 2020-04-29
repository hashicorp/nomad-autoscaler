package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadTarget "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/nomad/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Nomad Target plugin.
func factory(log hclog.Logger) interface{} {
	return nomadTarget.NewNomadPlugin(log)
}
