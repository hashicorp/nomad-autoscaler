package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	fixedvalue "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/fixed-value/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the FixedValue Strategy plugin.
func factory(log hclog.Logger) interface{} {
	return fixedvalue.NewFixedValuePlugin(log)
}
