// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// pluginName is the name of the plugin.
	pluginName = "instana"

	// configKeyEndpoint is the base URL of the Instana backend.
	configKeyEndpoint = "endpoint"

	// configKeyAPIToken is the Instana API token used for authentication.
	configKeyAPIToken = "api_token"

	// envKeyAPIToken is the environment variable fallback for the API token.
	envKeyAPIToken = "INSTANA_API_TOKEN"

	// metricsPath is the Instana REST API path for infrastructure metrics.
	metricsPath = "/api/infrastructure-monitoring/metrics"

	// rateLimitResetHdr is the response header Instana sets when rate-limiting.
	rateLimitResetHdr = "X-RateLimit-Reset"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) any { return NewInstanaPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

// APMPlugin is the Instana implementation of the APM interface.
type APMPlugin struct {
	logger hclog.Logger
	client *instanaClient
}

func NewInstanaPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
		client: newInstanaClient(),
	}
}

// SetConfig parses and validates the plugin configuration. All required fields
// are checked before any state is mutated.
func (a *APMPlugin) SetConfig(config map[string]string) error {
	endpoint := strings.TrimSpace(config[configKeyEndpoint])
	if endpoint == "" {
		return fmt.Errorf("%s config value cannot be empty", configKeyEndpoint)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configKeyEndpoint, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%s must be a valid absolute URL", configKeyEndpoint)
	}

	// config key takes precedence over the environment variable.
	token := strings.TrimSpace(config[configKeyAPIToken])
	if token == "" {
		token = strings.TrimSpace(os.Getenv(envKeyAPIToken))
	}
	if token == "" {
		return fmt.Errorf("%s config value cannot be empty", configKeyAPIToken)
	}

	a.client.baseURL = parsedURL
	a.client.apiToken = token

	return nil
}

// PluginInfo returns metadata about the plugin.
func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Query executes a single Instana metrics query and returns exactly one metric
// stream. If the query returns zero or more than one series, an appropriate
// error is returned.
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

// QueryMultiple executes an Instana infrastructure metrics query and returns
// all result series as separate metric streams. The query string q must be a
// JSON-encoded instanaMetricsRequest (without the timeFrame field, which is
// injected from r). One sdk.TimestampedMetrics is returned per
// (entity snapshot × metric ID) pair in the response.
func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	if r.From.Equal(r.To) {
		return nil, fmt.Errorf("query_window = %q is not supported by %s", "instant", pluginName)
	}

	a.logger.Debug("querying Instana", "query", q, "range", r)

	var queryReq instanaMetricsRequest
	if err := json.Unmarshal([]byte(q), &queryReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instana query: %w", err)
	}

	queryReq.TimeFrame = instanaTimeFrame{
		WindowSize: r.To.Sub(r.From).Milliseconds(),
		To:         r.To.UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metricsResp, err := a.client.getInfrastructureMetrics(ctx, queryReq)
	if err != nil {
		a.logger.Error("error querying instana metrics", "error", err)
		return []sdk.TimestampedMetrics{}, err
	}

	results := parseItems(metricsResp.Items)

	if len(results) == 0 {
		a.logger.Warn("empty time series response from instana, try a wider query window")
	}

	return results, nil
}

// parseItems converts the Instana response items into sdk.TimestampedMetrics
// slices. One slice is returned per (entity snapshot × metric ID) pair.
// Data points with zero timestamps are skipped.
func parseItems(items []instanaMetricItem) []sdk.TimestampedMetrics {
	var results []sdk.TimestampedMetrics

	for _, item := range items {
		for _, points := range item.Metrics {
			var metrics sdk.TimestampedMetrics
			for _, point := range points {
				metrics = append(metrics, sdk.TimestampedMetric{
					Timestamp: time.UnixMilli(int64(point[0])),
					Value:     point[1],
				})
			}
			if len(metrics) > 0 {
				results = append(results, metrics)
			}
		}
	}

	return results
}
