// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import (
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// getPluginMap converts the input plugin type to a plugin map that can be used
// when setting up a new plugin client.
func getPluginMap(pluginType string) map[string]plugin.Plugin {

	// All plugins should implement the base, so setup our map with this
	// already populated.
	m := map[string]plugin.Plugin{sdk.PluginTypeBase: &base.PluginBase{}}

	switch pluginType {
	case sdk.PluginTypeAPM:
		m[pluginType] = &apm.PluginAPM{}
	case sdk.PluginTypeTarget:
		m[pluginType] = &target.PluginTarget{}
	case sdk.PluginTypeStrategy:
		m[pluginType] = &strategy.PluginStrategy{}
	}
	return m
}
