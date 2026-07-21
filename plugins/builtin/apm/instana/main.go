// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	instana "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/instana/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Instana APM plugin.
func factory(log hclog.Logger) any {
	return instana.NewInstanaPlugin(log)
}
