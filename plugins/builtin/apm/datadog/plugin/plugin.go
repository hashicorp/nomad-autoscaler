package plugin

import (
	"context"
	"fmt"
	"net/http"
	"os"
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

	// naming convention according to datadog api
	envKeyClientAPIKey = "DD_CLIENT_API_KEY"
	envKeyClientAPPKey = "DD_CLIENT_APP_KEY"

	// Example string: FROM=1m;TO=0m;QUERY=<datadog_query>
	queryFromToken = "FROM="
	queryToToken   = "TO="
	queryToken     = "QUERY="
	queryDelim     = ";"

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

func (a *APMPlugin) Query(q string) (float64, error) {
	ddQuery, err := parseRawQuery(q)
	if err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(a.clientCtx, 10*time.Second)
	defer cancel()

	queryResult, res, err := a.client.MetricsApi.QueryMetrics(ctx).
		From(ddQuery.from.Unix()).
		To(ddQuery.to.Unix()).
		Query(ddQuery.query).
		Execute()
	if err != nil {
		return 0, fmt.Errorf("error querying metrics from datadog: %v", err)
	}

	if res.StatusCode == http.StatusTooManyRequests {
		return 0,
			fmt.Errorf("metric queries are ratelimited in current time period by datadog, resets in %s sec",
				res.Header.Get(ratelimitResetHdr))
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

func parseRawQuery(raw string) (datadogQuery, error) {
	// Split the input ddQuery to extract the window period
	querySplit := strings.Split(raw, ";")

	ddQuery := datadogQuery{}
	now := time.Now()
	for _, part := range querySplit {
		switch true {
		case strings.HasPrefix(part, queryFromToken):
			fromDur, err := time.ParseDuration(strings.TrimPrefix(part, queryFromToken))
			if err != nil {
				return ddQuery, fmt.Errorf("malformed %s window (Use go duration format): (%s) %v", queryFromToken, part, err)
			}
			ddQuery.from = now.Add(-fromDur)
		case strings.HasPrefix(part, queryToToken):
			//override to
			toDur, err := time.ParseDuration(strings.TrimPrefix(part, queryToToken))
			if err != nil {
				return ddQuery, fmt.Errorf("malformed %s window (Use go duration format): (%s) %v", queryToToken, part, err)
			}
			ddQuery.to = now.Add(-toDur)
		case strings.HasPrefix(part, queryToken):
			ddQuery.query = strings.TrimPrefix(part, queryToken)
		default:
			return ddQuery, fmt.Errorf("unrecognized field in check query string '%s'", part)
		}
	}

	// validations
	if len(ddQuery.query) == 0 {
		return ddQuery, fmt.Errorf("field %s cannot be empty. Supplied query: (%s)",
			queryToken, raw)
	}
	if ddQuery.to.IsZero() || ddQuery.from.IsZero() {
		return ddQuery, fmt.Errorf("fields %s, %s are required. Supplied query: %s",
			queryFromToken, queryToToken, raw)
	}
	if ddQuery.to.Sub(ddQuery.from) < 0 {
		return ddQuery, fmt.Errorf("field %s cannot have a time value before %s. Supplied query: %s",
			queryToToken, queryFromToken, raw)
	}

	return ddQuery, nil
}
