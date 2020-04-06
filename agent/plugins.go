package agent

import (
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
)

// setupPlugins is used to setup the plugin manager for all the agents plugins
// and forks the configured plugins for use.
func (a *Agent) setupPlugins() error {

	a.pluginManager = manager.NewPluginManager(a.logger, a.config.PluginDir, a.setupPluginConfig())

	// Trigger the loading of the plugins which will be available to the agent.
	// Any errors here will cause the agent to fail, but will include wrapped
	// errors so the user can fix any problems in a single iteration.
	return a.pluginManager.Load()
}

// setupPluginConfig builds a map which is used by the plugin manager to load
// all the configured plugins.
func (a *Agent) setupPluginConfig() map[string][]*config.Plugin {

	cfg := map[string][]*config.Plugin{}

	if len(a.config.APMs) > 0 {
		cfg[plugins.PluginTypeAPM] = a.config.APMs
	}
	if len(a.config.Strategies) > 0 {
		cfg[plugins.PluginTypeStrategy] = a.config.Strategies
	}
	if len(a.config.Targets) > 0 {
		cfg[plugins.PluginTypeTarget] = a.config.Targets
	}

	// Iterate the configurations to match any nomad plugins. If these plugins
	// have an empty config map, update this with the agents top level Nomad
	// config. This helps stop the need for repeating the same params over and
	// over.
	for _, cfgs := range cfg {
		for _, c := range cfgs {
			if (c.Name == plugins.InternalAPMNomad || c.Name == plugins.InternalTargetNomad) && len(c.Config) == 0 {
				c.Config = nomadHelper.ConfigToMap(a.config.Nomad)
			}
		}
	}

	return cfg
}
