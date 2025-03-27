package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	ibmCloudIG "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/ibmcloud-ig/plugin"
)

// factory returns a new instance of the IBMCloud Instance Group plugin.
func factory(log hclog.Logger) interface{} {
	return ibmCloudIG.NewIBMIGPlugin(log)
}

// This main function instantiates the same code as linked in as a builtin plugin.  This could be
// used to deploy just the plugin as an external plugin, but I deployed as builtin.
func main() {
	plugins.Serve(factory)
}
