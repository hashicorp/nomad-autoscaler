package manager

import (
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
)

// loadExternalPlugin takes the passed plugin and places it into the
// PluginManager store as a record that it should be launched and dispensed.
func (pm *PluginManager) loadExternalPlugin(cfg *config.Plugin, pluginType string) {

	info := &pluginInfo{
		args:    cfg.Args,
		config:  cfg.Config,
		exePath: filepath.Join(pm.pluginDir, cleanPluginExecutable(cfg.Name)),
	}

	// Add the plugin.
	pm.pluginsLock.Lock()
	pm.plugins[plugins.PluginID{Name: cfg.Name, PluginType: pluginType}] = info
	pm.pluginsLock.Unlock()

}

// cleanPluginExecutable is a helper function to remove commonly-found binary
// extensions which are not needed.
func cleanPluginExecutable(name string) string {
	switch {
	case strings.HasSuffix(name, ".exe"):
		return strings.TrimSuffix(name, ".exe")
	default:
		return name
	}
}
