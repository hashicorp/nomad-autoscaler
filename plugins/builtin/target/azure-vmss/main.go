// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	azure "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/azure-vmss/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the Azure VMSS plugin.
func factory(log hclog.Logger) interface{} {
	return azure.NewAzureVMSSPlugin(log)
}
