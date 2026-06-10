// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
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

// pluginConfig holds the validated, normalised plugin configuration.
// All values are trimmed and parsed once in parseConfig.
type pluginConfig struct {
	BaseURL  *url.URL
	APIToken string
}

// APMPlugin is the Instana implementation of the APM interface.
type APMPlugin struct {
	cfg    pluginConfig
	client *http.Client
	logger hclog.Logger
}

// NewInstanaPlugin returns a new Instana APM plugin instance.
func NewInstanaPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

// SetConfig parses and validates the plugin configuration. All required fields
// are checked before any state is mutated.
func (a *APMPlugin) SetConfig(config map[string]string) error {
	cfg, err := parseConfig(config)
	if err != nil {
		return err
	}

	a.cfg = cfg
	a.client = &http.Client{}

	return nil
}

// parseConfig validates the raw config map and returns a normalised
// pluginConfig. Returns an error if any required field is missing or invalid.
func parseConfig(config map[string]string) (pluginConfig, error) {
	var cfg pluginConfig

	endpoint := strings.TrimSpace(config[configKeyEndpoint])
	if endpoint == "" {
		return cfg, fmt.Errorf("%s config value cannot be empty", configKeyEndpoint)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return cfg, fmt.Errorf("failed to parse %s: %w", configKeyEndpoint, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return cfg, fmt.Errorf("%s must be a valid absolute URL", configKeyEndpoint)
	}

	cfg.BaseURL = parsedURL

	// config key takes precedence over the environment variable.
	cfg.APIToken = strings.TrimSpace(config[configKeyAPIToken])
	if cfg.APIToken == "" {
		cfg.APIToken = strings.TrimSpace(os.Getenv(envKeyAPIToken))
	}
	if cfg.APIToken == "" {
		return cfg, fmt.Errorf("%s config value cannot be empty", configKeyAPIToken)
	}

	return cfg, nil
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
// JSON-encoded instanaQueryRequest (without the timeFrame field, which is
// injected from r). One sdk.TimestampedMetrics is returned per
// (entity snapshot × metric ID) pair in the response.
func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	if r.From.Equal(r.To) {
		return nil, fmt.Errorf("query_window = %q is not supported by %s", "instant", pluginName)
	}

	a.logger.Debug("querying Instana", "query", q, "range", r)

	var queryReq instanaQueryRequest
	if err := json.Unmarshal([]byte(q), &queryReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instana query: %w", err)
	}

	queryReq.TimeFrame = instanaTimeFrame{
		WindowSize: r.To.Sub(r.From).Milliseconds(),
		To:         r.To.UnixMilli(),
	}

	body, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instana query request: %w", err)
	}

	metricsURL := *a.cfg.BaseURL
	metricsURL.Path = strings.TrimSuffix(metricsURL.Path, "/") + metricsPath

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metricsURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build instana query request: %w", err)
	}

	req.Header.Set("Authorization", "apiToken "+a.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error querying metrics from instana: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("metric queries are ratelimited by instana, resets at %s",
			resp.Header.Get(rateLimitResetHdr))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("instana query failed with status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var metricsResp instanaMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&metricsResp); err != nil {
		return nil, fmt.Errorf("failed to decode instana response: %w", err)
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

// instanaQueryRequest is the JSON body POSTed to the Instana infrastructure
// metrics endpoint: POST /api/infrastructure-monitoring/metrics
type instanaQueryRequest struct {
	TimeFrame   instanaTimeFrame `json:"timeFrame"`
	Plugin      string           `json:"plugin"`
	Query       string           `json:"query,omitempty"`
	SnapshotIDs []string         `json:"snapshotIds,omitempty"`
	Rollup      int32            `json:"rollup,omitempty"`
	Metrics     []string         `json:"metrics"`
}

// instanaTimeFrame defines the query window sent to Instana.
// Both fields are Unix millisecond epochs; WindowSize is To minus From.
type instanaTimeFrame struct {
	WindowSize int64 `json:"windowSize"`
	To         int64 `json:"to"`
}

// instanaMetricsResponse is the top-level JSON response from Instana.
type instanaMetricsResponse struct {
	Items []instanaMetricItem `json:"items"`
}

// instanaMetricItem represents one entity snapshot returned in the response.
// Metrics maps a metric ID to a slice of [timestamp_ms, value] pairs.
type instanaMetricItem struct {
	SnapshotID string                 `json:"snapshotId"`
	Label      string                 `json:"label"`
	Metrics    map[string][][2]float64 `json:"metrics"`
}
