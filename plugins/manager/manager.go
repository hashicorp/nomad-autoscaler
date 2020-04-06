package manager

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
)

// PluginManager is the brains of the plugin operation and should be used to
// manage plugin lifecycle as well as provide access to the underlying plugin
// interfaces.
type PluginManager struct {

	// cfg is our stored configuration of plugins to dispense. The format is
	// not ideal, but it works well and is easy to understand. The maps are
	// keyed by plugin type and plugin driver respectively.
	cfg map[string]map[string]*config.Plugin

	logger    hclog.Logger
	pluginDir string

	// pluginInstances are our dispensed plugins held as PluginInstance
	// wrappers.
	pluginInstancesLock sync.RWMutex
	pluginInstances     map[plugins.PluginID]PluginInstance

	// plugin contains all the information needed to launch and dispense the
	// Nomad Autoscaler plugins.
	pluginsLock sync.RWMutex
	plugins     map[plugins.PluginID]*pluginInfo
}

// pluginInfo contains all the required information to launch an Autoscaler
// plugin. Not all details are required depending on whether the plugin is
// internal or external; these are noted on the params.
type pluginInfo struct {
	baseInfo *plugins.PluginInfo
	config   map[string]string

	// args and exePath are required to execute the external plugin command.
	args    []string
	exePath string

	// factory is only populated when the plugin is internal.
	factory plugins.PluginFactory
}

// NewPluginManager sets up a new PluginManager for use.
func NewPluginManager(log hclog.Logger, dir string, cfg map[string][]*config.Plugin) *PluginManager {
	return &PluginManager{
		cfg:             generateConfigMap(cfg),
		logger:          log.Named("plugin-manager"),
		pluginDir:       dir,
		pluginInstances: make(map[plugins.PluginID]PluginInstance),
		plugins:         make(map[plugins.PluginID]*pluginInfo),
	}
}

// Load is responsible for registering and executing the plugins configured for
// use by the Autoscaler agent.
func (pm *PluginManager) Load() error {

	for t, cfgs := range pm.cfg {
		for _, cfg := range cfgs {

			// Figure out if the plugin is internal or external, then perform
			// the loading of the config into the manager store.
			if pm.useInternal(cfg.Driver) {
				pm.loadInternalPlugin(cfg, t)
			} else {
				pm.loadExternalPlugin(cfg, t)
			}
		}
	}

	return pm.dispensePlugins()
}

// KillPlugins calls Kill on all plugins currently dispensed.
func (pm *PluginManager) KillPlugins() {
	for id, v := range pm.pluginInstances {
		pm.logger.Info("shutting down plugin", "plugin_name", id.Name)
		v.Kill()
	}
}

// Dispense returns a PluginInstance for use by safely obtaining the
// PluginInstance from storage if we have it.
func (pm *PluginManager) Dispense(name, pluginType string) (PluginInstance, error) {
	pm.pluginInstancesLock.RLock()
	defer pm.pluginInstancesLock.RUnlock()

	// Attempt to pull our plugin instance from the store and pass this to the
	// caller.
	//
	// TODO(jrasell) if we do not find the instance, we should probably try and
	//  dispense the plugin. We should also check the plugin instance has not
	//  exited.
	inst, ok := pm.pluginInstances[plugins.PluginID{Name: name, PluginType: pluginType}]
	if !ok {
		return nil, fmt.Errorf("failed to dispense plugin: %s of type %s is not stored", name, pluginType)
	}
	return inst, nil
}

// dispensePlugins launches all configured plugins. It is responsible for
// executing external binaries as well as setting the config on all plugins so
// they are in a ready state. Any errors from this process will result in the
// agent failing on startup.
func (pm *PluginManager) dispensePlugins() error {

	// This function only gets called once currently; but it does perform all
	// the plugin loading based on the current configuration. Therefore for
	// future protection blat out our instances map.
	pm.pluginInstancesLock.Lock()
	pm.pluginInstances = make(map[plugins.PluginID]PluginInstance)
	pm.pluginInstancesLock.Unlock()

	var mErr multierror.Error

	pm.pluginsLock.Lock()
	defer pm.pluginsLock.Unlock()

	for pID, pInfo := range pm.plugins {

		var (
			inst PluginInstance
			info *plugins.PluginInfo
			err  error
		)
		if pInfo.factory != nil {
			inst, info, err = pm.launchInternalPlugin(pID, pInfo)
		} else {
			inst, info, err = pm.launchExternalPlugin(pID, pInfo)
		}

		// If we got an error dispensing the plugin, add this to the muilterror
		// and continue the loop.
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("failed to dispense plugin %s: %v", pID.Name, err))
			continue
		}

		// Update our tracking to detail the plugin base information returned
		// from the plugin itself.
		pm.plugins[pID].baseInfo = info

		// Perform the SetConfig on the plugin to ensure its state is as the
		// operator desires.
		if err := inst.Plugin().(base.Plugin).SetConfig(pInfo.config); err != nil {
			inst.Kill()
			_ = multierror.Append(&mErr, fmt.Errorf("failed to set config on plugin %s: %v", pID.Name, err))
			continue
		}

		// Store our plugin instance.
		pm.pluginInstancesLock.Lock()
		pm.pluginInstances[pID] = inst
		pm.pluginInstancesLock.Unlock()

		// When logging to INFO, the plugins do not log anything during startup
		// therefore log something useful to show the plugin is ready.
		pm.logger.Info("successfully launched and dispensed plugin", "plugin_name", pID.Name)
	}

	return mErr.ErrorOrNil()
}

// launchInternalPlugin is used to dispense internal plugins.
func (pm *PluginManager) launchInternalPlugin(id plugins.PluginID, info *pluginInfo) (PluginInstance, *plugins.PluginInfo, error) {

	raw := info.factory(pm.logger.ResetNamed("internal-plugin"))

	pInfo, err := pm.pluginLaunchCheck(id, raw)
	if err != nil {
		return nil, nil, err
	}
	return &internalPluginInstance{instance: raw}, pInfo, nil
}

// launchExternalPlugin is used to dispense external plugins. These plugins
// are therefore run as separate binaries and require more setup than internal
// ones.
func (pm *PluginManager) launchExternalPlugin(id plugins.PluginID, info *pluginInfo) (PluginInstance, *plugins.PluginInfo, error) {

	// Create a new client for the external plugin. This includes items such as
	// the command to execute and also the logger to use. The loggers name is
	// reset to avoid confusion that the log line is from within the agent.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins:         getPluginMap(id.PluginType),
		Cmd:             exec.Command(info.exePath, info.args...),
		Logger:          pm.logger.ResetNamed("external-plugin"),
	})

	// Connect via RPC.
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("failed to instantiate plugin %s client: %v", id.Name, err)
	}

	// Dispense a new instance of the external plugin.
	raw, err := rpcClient.Dispense(id.PluginType)
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("failed to dispense plugin %s: %v", id.Name, err)
	}

	pInfo, err := pm.pluginLaunchCheck(id, raw)
	if err != nil {
		client.Kill()
		return nil, nil, err
	}

	return &externalPluginInstance{instance: raw, client: client}, pInfo, nil
}

func (pm *PluginManager) pluginLaunchCheck(id plugins.PluginID, raw interface{}) (*plugins.PluginInfo, error) {

	// Check that the plugin implements the base plugin interface. As these are
	// external plugins we need to check this safely, otherwise an incorrect
	// plugin can cause the core application to panic.
	b, ok := raw.(base.Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement base plugin", id.Name)
	}

	pluginInfo, err := b.PluginInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to call PluginInfo on %s: %v", id.Name, err)
	}

	// If the plugin name, or plugin do not match it means the executed plugin
	// has returned its metadata that is different to the configured. This is a
	// problem, particularly in the PluginType sense as it means it will be
	// unable to fulfill its role.
	if pluginInfo.Name != id.Name || pluginInfo.PluginType != id.PluginType {
		return nil, fmt.Errorf("plugin %s remote info doesn't match local config: %v", id.Name, err)
	}

	return pluginInfo, nil
}
