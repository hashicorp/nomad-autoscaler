package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
)

const (
	// pluginName is the name of the plugin
	pluginName = "datadog"

	configKeyClientAPIKey = "dd_api_key"
	configKeyClientAPPKey = "dd_app_key"
)

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

	// Cannot proceed if the keys are unset
	if a.config[configKeyClientAPIKey] == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyClientAPIKey)
	}
	if a.config[configKeyClientAPPKey] == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyClientAPPKey)
	}

	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {
				Key: a.config[configKeyClientAPIKey],
			},
			"appKeyAuth": {
				Key: a.config[configKeyClientAPPKey],
			},
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

func (a *APMPlugin) Query(q string) (float64, error) {

	// Split the input query to extract the window period
	querySplit := strings.Split(q, ";")

	now := time.Now()
	from := int64(0)
	to := now.Unix()
	query := ""

	for _, part := range querySplit {
		switch true {
		case strings.HasPrefix(part, "FROM="):
			fromDur, err := time.ParseDuration(strings.TrimPrefix(part, "FROM="))
			if err != nil {
				return 0, fmt.Errorf("malformed from window: (%s) %v", part, err)
			}
			from = now.Add(-fromDur).Unix()
		case strings.HasPrefix(part, "TO="):
			//override to
			toDur, err := time.ParseDuration(strings.TrimPrefix(part, "TO="))
			if err != nil {
				return 0, fmt.Errorf("malformed to window: (%s) %v", part, err)
			}
			to = now.Add(-toDur).Unix()
		case strings.HasPrefix(part, "QUERY="):
			query = strings.TrimPrefix(part, "QUERY=")
		default:
			return 0, fmt.Errorf("unrecognized field in query string %s", part)
		}
	}

	if to < from {
		return 0, fmt.Errorf("TO=(%d) cannot be before FROM=(%d). Supplied query: %s", to, from, query)
	}

	ctx, cancel := context.WithTimeout(a.clientCtx, 10*time.Second)
	defer cancel()

	queryResult, _, err := a.client.MetricsApi.QueryMetrics(ctx).From(from).To(to).Query(query).Execute()
	if err != nil {
		return 0, fmt.Errorf("error querying metrics from datadog: %v", err)
	}

	// only support scalar types for now
	series := queryResult.GetSeries()
	if len(series) == 0 {
		return 0, fmt.Errorf("empty time series response from datadog, try a wider query window")
	}
	if pl, ok := series[0].GetPointlistOk(); ok {
		// pl is [[timestamp, value]...] array
		dataPoint := (*pl)[len(*pl)-1][1]
		return dataPoint, nil
	}
	return 0, fmt.Errorf("no data points found in time series response from datadog, try a wider query window")
}
