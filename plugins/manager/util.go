package manager

import (
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

// getPluginMap converts the input plugin type to a plugin map that can be used
// when setting up a new plugin client.
func getPluginMap(pluginType string) map[string]plugin.Plugin {
	m := make(map[string]plugin.Plugin, 1)
	switch pluginType {
	case plugins.PluginTypeAPM:
		m[pluginType] = &apm.Plugin{}
	case plugins.PluginTypeTarget:
		m[pluginType] = &target.Plugin{}
	case plugins.PluginTypeStrategy:
		m[pluginType] = &strategy.Plugin{}
	}
	return m
}
