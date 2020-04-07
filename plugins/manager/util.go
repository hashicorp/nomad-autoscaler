package manager

import (
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

// generateConfigMap translates the provide plugin configuration from the agent
// to the current structure used by the plugin manager.
func generateConfigMap(cfgs map[string][]*config.Plugin) map[string]map[string]*config.Plugin {
	pluginMapping := make(map[string]map[string]*config.Plugin)
	for t, cfg := range cfgs {
		if _, ok := pluginMapping[t]; !ok {
			pluginMapping[t] = make(map[string]*config.Plugin)
		}

		for _, c := range cfg {
			pluginMapping[t][c.Driver] = c
		}
	}
	return pluginMapping
}

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
