package manager

import (
	"os"
	"path"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadAPM "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/nomad/plugin"
	prometheus "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/prometheus/plugin"
	targetValue "github.com/hashicorp/nomad-autoscaler/plugins/builtin/strategy/target-value/plugin"
	awsASG "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/aws-asg/plugin"
	nomadTarget "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/nomad/plugin"
)

// loadInternalPlugin takes the plugin configuration and attempts to load it
// from internally to the plugin store.
func (pm *PluginManager) loadInternalPlugin(cfg *config.Plugin, pluginType string) {

	info := &pluginInfo{config: cfg.Config}

	switch cfg.Driver {
	case plugins.InternalAPMNomad:
		info.factory = nomadAPM.PluginConfig.Factory
		info.driver = "nomad-apm"
	case plugins.InternalTargetNomad:
		info.factory = nomadTarget.PluginConfig.Factory
		info.driver = "nomad-target"
	case plugins.InternalStrategyTargetValue:
		info.factory = targetValue.PluginConfig.Factory
		info.driver = "target-value"
	case plugins.InternalAPMPrometheus:
		info.factory = prometheus.PluginConfig.Factory
		info.driver = "prometheus"
	case plugins.InternalTargetAWSASG:
		info.factory = awsASG.PluginConfig.Factory
		info.driver = "aws-asg"
	default:
		pm.logger.Error("unsupported internal plugin", "plugin", cfg.Driver)
		return
	}

	pm.pluginsLock.Lock()
	pm.plugins[plugins.PluginID{Name: cfg.Name, PluginType: pluginType}] = info
	pm.pluginsLock.Unlock()

}

// useInternal decides whether we should use the internal implementation of the
// plugin. The preference is to use externally found plugins over the internal
// plugin.
func (pm *PluginManager) useInternal(plugin string) bool {

	// Create the full path to the intended plugin.
	filePath := path.Join(pm.pluginDir, plugin)

	// If the plugin binary is found locally on disk, use that rather than load
	// the plugin internally. This mainly benefits development effort, but also
	// provides general flexibility and known load ordering.
	if f, err := os.Stat(filePath); err == nil {

		// Ensure the named file is not a directory. If it is, then this isn't
		// the plugin executable we are looking for.
		//
		// Also Perform a basic check on the file/path. This depends on what OS
		// the agent is running. Check each util function to understand OS
		// specifics.
		if !f.IsDir() && executable(filePath, f) {
			return false
		}
	}

	switch plugin {
	case plugins.InternalAPMNomad,
		plugins.InternalTargetNomad,
		plugins.InternalAPMPrometheus,
		plugins.InternalStrategyTargetValue,
		plugins.InternalTargetAWSASG:
		return true
	default:
		return false
	}
}
