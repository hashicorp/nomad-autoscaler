// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	threshold "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/threshold/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Threshold Strategy plugin.
func factory(log hclog.Logger) interface{} {
	return threshold.NewThresholdPlugin(log)
}
