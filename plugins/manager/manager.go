// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import (
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

// PluginManager is the brains of the plugin operation and should be used to
// manage plugin lifecycle as well as provide access to the underlying plugin
// interfaces.
type PluginManager struct {

	// cfg is our stored configuration of plugins to dispense.
	cfg map[string][]*config.Plugin

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
	baseInfo *base.PluginInfo
	config   map[string]string

	// args and exePath are required to execute the external plugin command.
	driver  string
	args    []string
	exePath string

	// factory is only populated when the plugin is internal.
	factory plugins.PluginFactory
}

func setupPluginsConfig(agentCfg *config.Agent, apiConfig *api.Config) (map[string][]*config.Plugin, error) {
	cfg := map[string][]*config.Plugin{}

	if len(agentCfg.APMs) > 0 {
		cfg[sdk.PluginTypeAPM] = agentCfg.APMs
	}
	if len(agentCfg.Strategies) > 0 {
		cfg[sdk.PluginTypeStrategy] = agentCfg.Strategies
	}
	if len(agentCfg.Targets) > 0 {
		cfg[sdk.PluginTypeTarget] = agentCfg.Targets
	}

	// Iterate the configs and perform the config setup on each. If the
	// operator did not specify any config, it will be nil so make sure we
	// initialise the map.
	for _, cfgs := range cfg {
		for _, c := range cfgs {
			if c.Config == nil {
				c.Config = make(map[string]string)
			}
			err := setupPluginConfig(apiConfig, c.Config)
			if err != nil {
				fmt.Errorf("failed to config plugin: %w", err)
				return nil, err
			}

		}
	}

	return cfg, nil
}

// setupPluginConfig takes the individual plugin configuration and merges in
// namespaced Nomad configuration unless the user has disabled this
// functionality.
func setupPluginConfig(nomadCfg *api.Config, cfg map[string]string) error {

	// Look for the config flag that users can supply to toggle inheriting the
	// Nomad config from the agent. If we do not find it, opt-in by default.
	val, ok := cfg[plugins.ConfigKeyNomadConfigInherit]
	if !ok {
		nomadHelper.MergeMapWithAgentConfig(cfg, nomadCfg)
		return nil
	}

	// Attempt to convert the string. If the operator made an effort to
	// configure the key but got the value wrong, log the error and do not
	// perform the merge. The operator can fix the error and we do not make an
	// assumption.
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		fmt.Errorf("failed to convert config value to bool: %w", err)
		return err
	}

	if boolVal {
		nomadHelper.MergeMapWithAgentConfig(cfg, nomadCfg)
	}
	return nil
}

// NewPluginManager sets up a new PluginManager for use.
func NewPluginManager(log hclog.Logger, agentCfg *config.Agent, apiConfig *api.Config) (*PluginManager, error) {
	cfg, err := setupPluginsConfig(agentCfg, apiConfig)
	if err != nil {
		return nil, err
	}

	return &PluginManager{
		cfg:             cfg,
		logger:          log.Named("plugin_manager"),
		pluginDir:       agentCfg.PluginDir,
		pluginInstances: make(map[plugins.PluginID]PluginInstance),
		plugins:         make(map[plugins.PluginID]*pluginInfo),
	}, nil
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
			} else if isEnterprise(cfg.Driver) {
				pm.loadEnterprisePlugin(cfg, t)
			} else {
				pm.loadExternalPlugin(cfg, t)
			}
		}
	}

	return pm.dispensePlugins()
}

func (pm *PluginManager) Reload(newCfg map[string][]*config.Plugin) error {
	// Find plugins that are no longer in the new config and stop them.
	pluginsToStop := []plugins.PluginID{}

	for pType, cfgs := range pm.cfg {
		newCfgs := newCfg[pType]

		for _, plugin := range cfgs {
			stop := true
			for _, newPlugin := range newCfgs {
				if plugin.Name == newPlugin.Name {
					stop = false
					break
				}
			}

			if stop {
				pID := plugins.PluginID{PluginType: pType, Name: plugin.Name}
				pluginsToStop = append(pluginsToStop, pID)
			}
		}
	}

	for _, pID := range pluginsToStop {
		// Stop current plugin instance.
		pm.pluginInstancesLock.Lock()
		pm.killPluginLocked(pID)
		pm.pluginInstancesLock.Unlock()

		// And remove its configuration.
		pm.pluginsLock.Lock()
		delete(pm.plugins, pID)
		pm.pluginsLock.Unlock()
	}

	// We are only reading from the pluginInstances map, but we will likely
	// modify instances internals, so acquire a RWLock.
	pm.pluginInstancesLock.Lock()

	// Reload remaining plugins.
	var result error
	for t, cfgs := range newCfg {
		for _, p := range cfgs {
			pID := plugins.PluginID{PluginType: t, Name: p.Name}

			pInst, ok := pm.pluginInstances[pID]
			if !ok {
				continue
			}

			err := pInst.Plugin().(base.Base).SetConfig(p.Config)
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
	}
	pm.pluginInstancesLock.Unlock()

	// Store new config and call Load() to start new plugins.
	pm.cfg = newCfg
	err := pm.Load()
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result
}

// KillPlugins calls Kill on all plugins currently dispensed.
func (pm *PluginManager) KillPlugins() {
	pm.pluginInstancesLock.Lock()
	defer pm.pluginInstancesLock.Unlock()

	for id := range pm.pluginInstances {
		pm.killPluginLocked(id)
	}
}

// killPlugin stops a specific plugin and removes it from the manager.
// A lock for pm.pluginInstancesLock must be acquired before calling this method.
func (pm *PluginManager) killPluginLocked(pID plugins.PluginID) {
	p, ok := pm.pluginInstances[pID]
	if !ok {
		return
	}

	pm.logger.Info("shutting down plugin", "plugin_name", pID.Name)
	p.Kill()
	delete(pm.pluginInstances, pID)
}

// Dispense returns a PluginInstance for use by safely obtaining the
// PluginInstance from storage if we have it.
func (pm *PluginManager) Dispense(name, pluginType string) (PluginInstance, error) {

	// Measure the time taken to dispense a plugin. This helps identify
	// contention and pressure obtaining plugin client handles.
	labels := []metrics.Label{{Name: "plugin_name", Value: name}, {Name: "plugin_type", Value: pluginType}}
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "manager", "access_ms"}, time.Now(), labels)

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
		return nil, fmt.Errorf("failed to dispense plugin: %q of type %q is not stored", name, pluginType)
	}
	return inst, nil
}

// dispensePlugins launches all configured plugins. It is responsible for
// executing external binaries as well as setting the config on all plugins so
// they are in a ready state. Any errors from this process will result in the
// agent failing on startup.
func (pm *PluginManager) dispensePlugins() error {
	var mErr multierror.Error

	for pID, pInfo := range pm.plugins {
		// Skip if plugin is already dispensed.
		pm.pluginInstancesLock.Lock()
		_, ok := pm.pluginInstances[pID]
		pm.pluginInstancesLock.Unlock()
		if ok {
			continue
		}

		var (
			inst PluginInstance
			info *base.PluginInfo
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
		pm.pluginsLock.Lock()
		pm.plugins[pID].baseInfo = info
		pm.pluginsLock.Unlock()

		// Perform the SetConfig on the plugin to ensure its state is as the
		// operator desires.
		if err := inst.Plugin().(base.Base).SetConfig(pInfo.config); err != nil {
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
func (pm *PluginManager) launchInternalPlugin(id plugins.PluginID, info *pluginInfo) (PluginInstance, *base.PluginInfo, error) {

	raw := info.factory(pm.logger.ResetNamed("internal_plugin." + id.Name))

	pInfo, err := pm.pluginLaunchCheck(id, info, raw)
	if err != nil {
		return nil, nil, err
	}
	return &internalPluginInstance{instance: raw}, pInfo, nil
}

// launchExternalPlugin is used to dispense external plugins. These plugins
// are therefore run as separate binaries and require more setup than internal
// ones.
func (pm *PluginManager) launchExternalPlugin(id plugins.PluginID, info *pluginInfo) (PluginInstance, *base.PluginInfo, error) {

	// Create a new client for the external plugin. This includes items such as
	// the command to execute and also the logger to use. The loggers name is
	// reset to avoid confusion that the log line is from within the agent.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  plugins.Handshake,
		Plugins:          getPluginMap(id.PluginType),
		Cmd:              exec.Command(info.exePath, info.args...),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           pm.logger.ResetNamed("external_plugin"),
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

	pInfo, err := pm.pluginLaunchCheck(id, info, raw)
	if err != nil {
		client.Kill()
		return nil, nil, err
	}

	return &externalPluginInstance{instance: raw, client: client}, pInfo, nil
}

func (pm *PluginManager) pluginLaunchCheck(id plugins.PluginID, info *pluginInfo, raw interface{}) (*base.PluginInfo, error) {

	// Check that the plugin implements the base plugin interface. As these are
	// external plugins we need to check this safely, otherwise an incorrect
	// plugin can cause the core application to panic.
	b, ok := raw.(base.Base)
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
	if pluginInfo.Name != info.driver || pluginInfo.PluginType != id.PluginType {
		return nil, fmt.Errorf("plugin %s remote info doesn't match local config: %v", id.Name, err)
	}

	return pluginInfo, nil
}
