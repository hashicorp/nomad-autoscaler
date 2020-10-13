package agent

import (
	"strconv"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
)

// setupPlugins is used to setup the plugin manager for all the agents plugins
// and forks the configured plugins for use.
func (a *Agent) setupPlugins() error {

	a.pluginManager = manager.NewPluginManager(a.logger, a.config.PluginDir, a.setupPluginsConfig())

	// Trigger the loading of the plugins which will be available to the agent.
	// Any errors here will cause the agent to fail, but will include wrapped
	// errors so the user can fix any problems in a single iteration.
	return a.pluginManager.Load()
}

// setupPluginsConfig builds a map which is used by the plugin manager to load
// all the configured plugins.
func (a *Agent) setupPluginsConfig() map[string][]*config.Plugin {

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

	// Iterate the configs and perform the config setup on each. If the
	// operator did not specify any config, it will be nil so make sure we
	// initialise the map.
	for _, cfgs := range cfg {
		for _, c := range cfgs {
			if c.Config == nil {
				c.Config = make(map[string]string)
			}
			a.setupPluginConfig(c.Config)
		}
	}

	return cfg
}

// setupPluginConfig takes the individual plugin configuration and merges in
// namespaced Nomad configuration unless the user has disabled this
// functionality.
func (a *Agent) setupPluginConfig(cfg map[string]string) {

	// Look for the config flag that users can supply to toggle inheriting the
	// Nomad config from the agent. If we do not find it, opt-in by default.
	val, ok := cfg[plugins.ConfigKeyNomadConfigInherit]
	if !ok {
		nomadHelper.MergeMapWithAgentConfig(cfg, a.config.Nomad)
		return
	}

	// Attempt to convert the string. If the operator made an effort to
	// configure the key but got the value wrong, log the error and do not
	// perform the merge. The operator can fix the error and we do not make an
	// assumption.
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		a.logger.Error("failed to convert config value to bool", "error", err)
		return
	}
	if boolVal {
		nomadHelper.MergeMapWithAgentConfig(cfg, a.config.Nomad)
	}
}

func (a *Agent) getNomadAPMNames() []string {
	var names []string
	for _, apm := range a.config.APMs {
		if apm.Driver == plugins.InternalAPMNomad {
			names = append(names, apm.Name)
		}
	}
	return names
}
