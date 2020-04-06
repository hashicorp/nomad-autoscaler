package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	// pluginName is the name of the plugin
	pluginName = "prometheus"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewPrometheusPlugin(l) },
	}

	pluginInfo = &plugins.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}
)

type APMPlugin struct {
	client api.Client
	config map[string]string
	logger hclog.Logger
}

func NewPrometheusPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

func (a *APMPlugin) SetConfig(config map[string]string) error {

	a.config = config

	promCfg := api.Config{
		Address: a.config["address"],
	}

	// create Prometheus client
	client, err := api.NewClient(promCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Prometheus client: %v", err)
	}

	// store config and client in plugin instance
	a.client = client

	return nil
}

func (a *APMPlugin) PluginInfo() (*plugins.PluginInfo, error) {
	return pluginInfo, nil
}

func (a *APMPlugin) Query(q string) (float64, error) {
	v1api := v1.NewAPI(a.client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, q, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to query: %v", err)
	}
	for _, w := range warnings {
		fmt.Printf("[WARN] %s", w)
	}

	// only support scalar types for now
	t := result.Type()
	if t != model.ValScalar {
		return 0, fmt.Errorf("result type (`%v`) is not `scalar`", t)
	}

	s := result.(*model.Scalar)
	return float64(s.Value), nil
}
