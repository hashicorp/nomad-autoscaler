package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	hcloud "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/hcloud-server/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the AWS ASG plugin.
func factory(log hclog.Logger) interface{} {
	return hcloud.NewHCloudServerPlugin(log)
}
