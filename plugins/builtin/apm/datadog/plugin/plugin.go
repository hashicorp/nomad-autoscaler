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

	// configKeySite is used to change the Datadog site
	configKeySite = "site"

	configKeyClientAPIKey = "dd_api_key"
	configKeyClientAPPKey = "dd_app_key"

	// naming convention according to datadog api
	envKeyClientAPIKey = "DD_API_KEY"
	envKeyClientAPPKey = "DD_APP_KEY"

	datadogAuthAPIKey = "apiKeyAuth"
	datadogAuthAPPKey = "appKeyAuth"

	ratelimitResetHdr = "X-Ratelimit-Reset"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewDatadogPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

type APMPlugin struct {
	client    *datadog.APIClient
	clientCtx context.Context
	config    map[string]string
	logger    hclog.Logger

	// ddConfigCallback is used to customize the Datadog client for testing.
	ddConfigCallback func(*datadog.Configuration)
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

	// set Datadog site if provided
	if a.config[configKeySite] != "" {
		ctx = context.WithValue(ctx,
			datadog.ContextServerVariables,
			map[string]string{
				"site": a.config[configKeySite],
			})
	}

	a.clientCtx = ctx

	// configure the Datadog API client.
	// Call the ddConfigCallback if provided to setup test harness.
	configuration := datadog.NewConfiguration()
	if a.ddConfigCallback != nil {
		a.ddConfigCallback(configuration)
	}

	// store config and client in plugin instance
	client := datadog.NewAPIClient(configuration)
	a.client = client

	return nil
}

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (a *APMPlugin) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	m, err := a.QueryMultiple(q, r)
	if err != nil {
		return nil, err
	}

	switch len(m) {
	case 0:
		return sdk.TimestampedMetrics{}, nil
	case 1:
		return m[0], nil
	default:
		return nil, fmt.Errorf("query returned %d metric streams, only 1 is expected", len(m))
	}
}

func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	ctx, cancel := context.WithTimeout(a.clientCtx, 10*time.Second)
	defer cancel()

	queryResult, res, err := a.client.MetricsApi.QueryMetrics(ctx, r.From.Unix(), r.To.Unix(), q)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusTooManyRequests {
			return nil,
				fmt.Errorf("metric queries are ratelimited in current time period by datadog, resets in %s sec",
					res.Header.Get(ratelimitResetHdr))
		}
		return nil, fmt.Errorf("error querying metrics from datadog: %v", err)
	}

	series := queryResult.GetSeries()
	if len(series) == 0 {
		a.logger.Warn("empty time series response from datadog, try a wider query window")
		return nil, nil
	}

	var results []sdk.TimestampedMetrics
	for _, s := range series {
		pl, ok := s.GetPointlistOk()
		if !ok {
			continue
		}

		var result sdk.TimestampedMetrics

		// pl is [[timestamp, value]...] array
		for _, p := range *pl {
			if len(p) != 2 {
				continue
			}

			ts := int64(*p[0]) / 1e3
			value := *p[1]
			tm := sdk.TimestampedMetric{
				Timestamp: time.Unix(ts, 0),
				Value:     value,
			}
			result = append(result, tm)
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		a.logger.Warn("no data points found in time series response from datadog, try a wider query window")
	}

	return results, nil
}
