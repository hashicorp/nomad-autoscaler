package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	osSenlin "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/openstack-senlin/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the OpenStack Senlin plugin.
func factory(log hclog.Logger) interface{} {
	return osSenlin.NewOSSenlinPlugin(log)
}
