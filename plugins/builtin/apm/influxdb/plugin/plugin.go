// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "influxdb"

	configKeyAddress  = "address"

	// Name of the InfluxDB database to query.
	configKeyDatabase = "database"

	// "db" is accepted as a shorthand for "database".
	configKeyDB = "db"

	// configKeyUsername is the username for Basic or JWT authentication.
	configKeyUsername = "username"

	// configKeyPassword is the password for Basic authentication. Mutually exclusive with shared_secret.
	configKeyPassword = "password"

	// configKeyVersion selects the InfluxDB API version. Only "1" is
	// currently supported; "3" is reserved for future implementation.
	configKeyVersion = "version"

	// configVersion1 is the default and only supported version today.
	configVersion1 = "1"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewInfluxDBPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

// influxQueryResponse models the top-level JSON returned by the InfluxDB 1.x
// /query HTTP endpoint.
type influxQueryResponse struct {
	Results []influxQueryResult `json:"results"`
	Error   string              `json:"error,omitempty"`
}

type influxQueryResult struct {
	StatementID int                 `json:"statement_id"`
	Series      []influxQuerySeries `json:"series"`
	Error       string              `json:"error,omitempty"`
}

type influxQuerySeries struct {
	Name    string          `json:"name"`
	Columns []string        `json:"columns"`
	Values  [][]interface{} `json:"values"`
}

// pluginConfig holds the validated, normalised plugin configuration.
// All values are trimmed and parsed once in SetConfig.
type pluginConfig struct {
	Address      string
	Database     string
	Username     string
	Password     string
	SharedSecret string
	TokenTTL     time.Duration // parsed JWT lifetime; defaults to 1h
}

// APMPlugin is the InfluxDB implementation of the APM interface.
type APMPlugin struct {
	cfg         pluginConfig
	client      *http.Client
	baseURL     *url.URL
	logger      hclog.Logger
	jwtMu       sync.Mutex // protects cachedToken and tokenExpiry
	cachedToken string     // most recently generated JWT; empty until first use
	tokenExpiry time.Time  // when cachedToken expires
}

// NewInfluxDBPlugin returns a new InfluxDB APM plugin instance.
func NewInfluxDBPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

// SetConfig parses and validates the plugin configuration. All required fields
// are checked before any state is mutated, so a failed re-configuration leaves
// the plugin in its previous working state.
func (a *APMPlugin) SetConfig(config map[string]string) error {
	cfg, baseURL, err := parseConfig(config)
	if err != nil {
		return err
	}

	a.cfg = cfg
	a.baseURL = baseURL
	a.client = &http.Client{}

	// Invalidate cached JWT on reconfigure.
	a.jwtMu.Lock()
	a.cachedToken = ""
	a.tokenExpiry = time.Time{}
	a.jwtMu.Unlock()

	return nil
}

// parseConfig validates the raw config map and returns the normalised
// pluginConfig and its parsed base URL. Returns an error if any required
// field is missing, a field value is invalid, or auth options conflict.
func parseConfig(config map[string]string) (pluginConfig, *url.URL, error) {
	var cfg pluginConfig

	cfg.Address = strings.TrimSpace(config[configKeyAddress])
	if cfg.Address == "" {
		return pluginConfig{}, nil, fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	cfg.Database = strings.TrimSpace(config[configKeyDatabase])
	if cfg.Database == "" {
		cfg.Database = strings.TrimSpace(config[configKeyDB])
	}
	if cfg.Database == "" {
		return pluginConfig{}, nil, fmt.Errorf("%q config value cannot be empty", configKeyDatabase)
	}

	cfg.Username = strings.TrimSpace(config[configKeyUsername])
	cfg.Password = strings.TrimSpace(config[configKeyPassword])
	cfg.SharedSecret = strings.TrimSpace(config[configKeySharedSecret])

	if cfg.SharedSecret != "" {
		if cfg.Username == "" {
			return pluginConfig{}, nil, fmt.Errorf("auth configuration error: %q requires %q (used as the JWT username claim)", configKeySharedSecret, configKeyUsername)
		}
		if cfg.Password != "" {
			return pluginConfig{}, nil, fmt.Errorf("conflicting auth configuration: %q cannot be used together with %q", configKeySharedSecret, configKeyPassword)
		}
	}

	cfg.TokenTTL = defaultTokenTTL
	if raw := strings.TrimSpace(config[configKeyTokenTTL]); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return pluginConfig{}, nil, fmt.Errorf("invalid %q value %q: %w", configKeyTokenTTL, raw, err)
		}
		if parsed < minTokenTTL || parsed > maxTokenTTL {
			return pluginConfig{}, nil, fmt.Errorf("invalid %q value %q: must be between %s and %s", configKeyTokenTTL, raw, minTokenTTL, maxTokenTTL)
		}
		cfg.TokenTTL = parsed
	}

	version := strings.TrimSpace(config[configKeyVersion])
	switch version {
	case "", configVersion1:
		// ok — v1 is the default
	case "2", "3":
		return pluginConfig{}, nil, fmt.Errorf("influxdb version %q is not yet supported: only version %q is currently implemented", version, configVersion1)
	default:
		return pluginConfig{}, nil, fmt.Errorf("invalid influxdb version %q: only version %q is supported", version, configVersion1)
	}

	parsedURL, err := url.Parse(cfg.Address)
	if err != nil {
		return pluginConfig{}, nil, fmt.Errorf("failed to parse %q: %w", configKeyAddress, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return pluginConfig{}, nil, fmt.Errorf("%q must be a valid absolute URL", configKeyAddress)
	}

	return cfg, parsedURL, nil
}

// PluginInfo returns metadata about the plugin.
func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Query executes a single InfluxQL query and returns exactly one metric
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

// QueryMultiple executes an InfluxQL query against the InfluxDB 1.x /query
// endpoint and returns all result series as separate metric streams.
//
// Note: Time filtering must be explicitly included in your InfluxQL query.
// The timeRange parameter is logged but not automatically applied.
// Example: "SELECT mean(cpu) FROM metrics WHERE time >= now() - 10m"
func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	if r.From.Equal(r.To) {
		return nil, fmt.Errorf("query_window = %q is not supported by %s", "instant", pluginName)
	}

	a.logger.Debug("querying InfluxDB", "query", q, "range", r)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryURL := *a.baseURL
	queryURL.Path = strings.TrimSuffix(queryURL.Path, "/") + "/query"

	queryValues := queryURL.Query()
	queryValues.Set("db", a.cfg.Database)
	queryValues.Set("q", q)
	queryValues.Set("epoch", "s")
	queryURL.RawQuery = queryValues.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build influxdb query request: %w", err)
	}

	if err := a.setAuthHeader(req); err != nil {
		return nil, fmt.Errorf("failed to set auth header: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error querying metrics from influxdb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("influxdb query failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var queryResp influxQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode influxdb query response: %w", err)
	}

	if queryResp.Error != "" {
		return nil, fmt.Errorf("influxdb query error: %s", queryResp.Error)
	}

	results := make([]sdk.TimestampedMetrics, 0)
	for _, result := range queryResp.Results {
		if result.Error != "" {
			return nil, fmt.Errorf("influxdb query error: %s", result.Error)
		}

		for _, series := range result.Series {
			metrics, err := parseSeries(series)
			if err != nil {
				return nil, err
			}
			if len(metrics) > 0 {
				results = append(results, metrics)
			}
		}
	}

	if len(results) == 0 {
		a.logger.Warn("empty time series response from influxdb, try a wider query window")
	}

	return results, nil
}

// parseSeries converts an InfluxDB result series into sdk.TimestampedMetrics.
// It looks for the "time" column and picks the first non-time column as the
// metric value, preferring a column named "value" if one exists.
func parseSeries(series influxQuerySeries) (sdk.TimestampedMetrics, error) {
	timeIdx := indexOfColumn(series.Columns, "time")
	if timeIdx == -1 {
		return nil, fmt.Errorf("influxdb series %q does not contain time column", series.Name)
	}

	valueIdx := indexOfColumn(series.Columns, "value")
	if valueIdx == -1 {
		// Fall back to the first non-time column.
		for i, col := range series.Columns {
			if col != "time" {
				valueIdx = i
				break
			}
		}
	}
	if valueIdx == -1 {
		return nil, fmt.Errorf("influxdb series %q does not contain a metric value column", series.Name)
	}

	metrics := make(sdk.TimestampedMetrics, 0, len(series.Values))
	for _, row := range series.Values {
		if timeIdx >= len(row) || valueIdx >= len(row) {
			continue
		}
		if row[timeIdx] == nil || row[valueIdx] == nil {
			continue
		}

		ts, err := parseEpochSeconds(row[timeIdx])
		if err != nil {
			continue
		}

		val, err := parseFloatValue(row[valueIdx])
		if err != nil {
			continue
		}

		metrics = append(metrics, sdk.TimestampedMetric{
			Timestamp: time.Unix(ts, 0),
			Value:     val,
		})
	}

	return metrics, nil
}

// parseEpochSeconds converts an interface{} (typically a JSON number) into an
// int64 Unix epoch in seconds.
func parseEpochSeconds(v interface{}) (int64, error) {
	switch t := v.(type) {
	case float64:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		parsed, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported timestamp type %T", v)
	}
}

// parseFloatValue converts an interface{} (typically a JSON number) into a
// float64 metric value.
func parseFloatValue(v interface{}) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case json.Number:
		return t.Float64()
	case string:
		return strconv.ParseFloat(t, 64)
	default:
		return 0, fmt.Errorf("unsupported value type %T", v)
	}
}

// indexOfColumn returns the index of the named column, or -1 if not found.
func indexOfColumn(columns []string, key string) int {
	for i, col := range columns {
		if col == key {
			return i
		}
	}
	return -1
}
