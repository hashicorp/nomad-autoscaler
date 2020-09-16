package plugin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// pluginName is the name of the plugin
	pluginName = "datadog"

	configKeyClientAPIKey = "dd_api_key"
	configKeyClientAPPKey = "dd_app_key"

	// naming convention according to datadog api
	envKeyClientAPIKey = "DD_API_KEY"
	envKeyClientAPPKey = "DD_APP_KEY"

	datadogAuthAPIKey = "apiKeyAuth"
	datadogAuthAPPKey = "appKeyAuth"

	ratelimitResetHdr = "X-Ratelimit-Reset"
)

type datadogQuery struct {
	from  time.Time
	to    time.Time
	query string
}

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewDatadogPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}
)

type APMPlugin struct {
	client    *datadog.APIClient
	clientCtx context.Context
	config    map[string]string
	logger    hclog.Logger
}

func NewDatadogPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

func (a *APMPlugin) SetConfig(config map[string]string) error {
	a.config = config

	// config keys override env keys
	if a.config[configKeyClientAPIKey] == "" {
		envAPIKey, ok := os.LookupEnv(envKeyClientAPIKey)
		if !ok || envAPIKey == "" {
			return fmt.Errorf("%q config value cannot be empty", configKeyClientAPIKey)
		}
		a.config[configKeyClientAPIKey] = envAPIKey
	}
	if a.config[configKeyClientAPPKey] == "" {
		envAPPKey, ok := os.LookupEnv(envKeyClientAPPKey)
		if !ok || envAPPKey == "" {
			return fmt.Errorf("%q config value cannot be empty", configKeyClientAPPKey)
		}
		a.config[configKeyClientAPPKey] = envAPPKey
	}

	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			datadogAuthAPIKey: {Key: a.config[configKeyClientAPIKey]},
			datadogAuthAPPKey: {Key: a.config[configKeyClientAPPKey]},
		},
	)
	a.clientCtx = ctx
	configuration := datadog.NewConfiguration()
	client := datadog.NewAPIClient(configuration)

	// store config and client in plugin instance
	a.client = client

	return nil
}

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (a *APMPlugin) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	ctx, cancel := context.WithTimeout(a.clientCtx, 10*time.Second)
	defer cancel()

	queryResult, res, err := a.client.MetricsApi.QueryMetrics(ctx).
		From(r.From.Unix()).
		To(r.To.Unix()).
		Query(q).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("error querying metrics from datadog: %v", err)
	}

	if res.StatusCode == http.StatusTooManyRequests {
		return nil,
			fmt.Errorf("metric queries are ratelimited in current time period by datadog, resets in %s sec",
				res.Header.Get(ratelimitResetHdr))
	}

	series := queryResult.GetSeries()
	if len(series) == 0 {
		a.logger.Warn("empty time series response from datadog, try a wider query window")
		return nil, nil
	}

	pl, ok := series[0].GetPointlistOk()
	if !ok {
		a.logger.Warn("no data points found in time series response from datadog, try a wider query window")
		return nil, nil
	}

	var result sdk.TimestampedMetrics

	// pl is [[timestamp, value]...] array
	for _, p := range *pl {
		tm := sdk.TimestampedMetric{
			Timestamp: time.Unix(int64(p[0]), 0),
			Value:     p[1],
		}
		result = append(result, tm)
	}

	return result, nil
}
