package plugin

import (
	"context"
	"fmt"
	"math"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	// pluginName is the name of the plugin
	pluginName = "prometheus"

	// configKeyAddress is the accepted configuration key which holds the
	// address param.
	configKeyAddress = "address"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewPrometheusPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
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

	// If the address is not set, or is empty within the config, any client
	// calls will fail. It seems logical to catch this here rather than just
	// let queries fail.
	if a.config[configKeyAddress] == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	promCfg := api.Config{
		Address: a.config[configKeyAddress],
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

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
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

	// If Prometheus returned warnings, report these to the user.
	for _, w := range warnings {
		a.logger.Warn("prometheus query returned warning", "warning", w)
	}

	// only support scalar types for now
	t := result.Type()
	if t != model.ValScalar {
		return 0, fmt.Errorf("result type (`%v`) is not `scalar`", t)
	}

	// Grab the Value from the result object and convert to a float64.
	floatVal := float64(result.(*model.Scalar).Value)

	// Check whether floatVal is an IEEE 754 not-a-number value. If it is
	// return an error.
	if math.IsNaN(floatVal) {
		return 0, fmt.Errorf("query result value is not-a-number")
	}

	return floatVal, nil
}
