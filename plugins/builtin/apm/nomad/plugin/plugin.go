// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"maps"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nomad-apm"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewNomadPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

type APMPlugin struct {
	mu     sync.RWMutex
	config map[string]string
	client *api.Client
	logger hclog.Logger
}

func NewNomadPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

func (a *APMPlugin) SetConfig(config map[string]string) error {
	configCopy := copyConfigMap(config)

	cfg := nomadHelper.ConfigFromNamespacedMap(configCopy)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	a.mu.Lock()
	a.config = configCopy
	a.client = client
	a.mu.Unlock()

	return nil
}

func (a *APMPlugin) getClientAndConfigSnapshot() (*api.Client, map[string]string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.client, copyConfigMap(a.config)
}

func copyConfigMap(config map[string]string) map[string]string {
	if config == nil {
		return nil
	}

	configCopy := make(map[string]string, len(config))
	maps.Copy(configCopy, config)

	return configCopy
}

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}
