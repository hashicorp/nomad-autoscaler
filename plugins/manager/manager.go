// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
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

// NewPluginManager sets up a new PluginManager for use.
func NewPluginManager(log hclog.Logger, dir string, cfg map[string][]*config.Plugin) *PluginManager {
	return &PluginManager{
		cfg:             cfg,
		logger:          log.Named("plugin_manager"),
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

	pID := plugins.PluginID{Name: name, PluginType: pluginType}

	// Attempt to pull our plugin instance from the store and pass this to the
	// caller. If the plugin isn't in the store, try to instantiate it; if it
	// is, do a health check and attempt to re-instantiate if it fails.
	pm.pluginInstancesLock.Lock()
	inst, exists := pm.pluginInstances[pID]

	if exists {
		if _, err := pm.pluginInfo(pID, inst); err == nil {
			// Plugin is fine, return it
			pm.pluginInstancesLock.Unlock()
			return inst, nil
		}

		// The plugin exists, but is broken. Remove it.
		pm.logger.Warn("plugin failed healthcheck", "plugin_name", pID.Name)
		pm.killPluginLocked(pID)
	} else {
		pm.logger.Warn("plugin not in store", "plugin_name", pID.Name)
	}
	pm.pluginInstancesLock.Unlock()

	if err := pm.dispensePlugins(); err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %q of type %q: %w", name, pluginType, err)
	}

	pm.pluginInstancesLock.RLock()
	inst, ok := pm.pluginInstances[pID]
	pm.pluginInstancesLock.RUnlock()
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
	pluginInfo, err := pm.pluginInfo(id, raw)
	if err != nil {
		return nil, err
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

func (pm *PluginManager) pluginInfo(id plugins.PluginID, raw interface{}) (*base.PluginInfo, error) {
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

	return pluginInfo, nil
}

func (pm *PluginManager) GetTarget(target *sdk.ScalingPolicyTarget) (targetpkg.Target, error) {
	// Dispense an instance of target plugin used by the policy.
	targetPlugin, err := pm.Dispense(target.Name, sdk.PluginTypeTarget)
	if err != nil {
		return nil, err
	}

	targetInst, ok := targetPlugin.Plugin().(targetpkg.Target)
	if !ok {
		err := fmt.Errorf("plugin %s (%T) is not a target plugin", target.Name, targetPlugin.Plugin())
		return nil, err
	}

	return targetInst, nil
}

func (pm *PluginManager) GetAPM(source string) (apm.APM, error) {
	// Dispense plugins.
	apmPlugin, err := pm.Dispense(source, sdk.PluginTypeAPM)
	if err != nil {
		return nil, fmt.Errorf(`apm plugin "%s" not initialized: %v`, source, err)
	}
	apmInst, ok := apmPlugin.Plugin().(apm.APM)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not an APM plugin`, source)
	}
	return apmInst, nil
}

func (pm *PluginManager) GetStrategy(name string) (strategy.Strategy, error) {
	strategyPlugin, err := pm.Dispense(name, sdk.PluginTypeStrategy)
	if err != nil {
		return nil, fmt.Errorf(`strategy plugin "%s" not initialized: %v`, name, err)
	}

	strategyInst, ok := strategyPlugin.Plugin().(strategy.Strategy)
	if !ok {
		return nil, fmt.Errorf(`"%s" is not a strategy plugin`, name)
	}
	return strategyInst, nil
}
