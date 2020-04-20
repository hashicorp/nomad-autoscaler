package plugin

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nomad-apm"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewNomadPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}
)

type APMPlugin struct {
	client *api.Client
	logger hclog.Logger
}

func NewNomadPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

func (a *APMPlugin) SetConfig(config map[string]string) error {

	cfg := nomadHelper.ConfigFromMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	a.client = client

	return nil
}

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}
