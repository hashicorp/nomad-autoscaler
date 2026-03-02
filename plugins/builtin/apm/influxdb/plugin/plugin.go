// Copyright IBM Corp. 2020, 2025
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
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// pluginName is the name of the plugin.
	pluginName = "influxdb"

	// configKeyAddress is the InfluxDB server address.
	configKeyAddress = "address"

	// configKeyDatabase is the primary config key for the database name.
	configKeyDatabase = "database"

	// configKeyDB is the shorthand alias accepted for database name.
	configKeyDB = "db"

	// configKeyUsername is the optional authentication username.
	configKeyUsername = "username"

	// configKeyPassword is the optional authentication password.
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
	StatementID int                  `json:"statement_id"`
	Series      []influxQuerySeries  `json:"series"`
	Error       string               `json:"error,omitempty"`
}

type influxQuerySeries struct {
	Name    string            `json:"name"`
	Columns []string          `json:"columns"`
	Values  [][]interface{}   `json:"values"`
}

// APMPlugin is the InfluxDB implementation of the APM interface.
type APMPlugin struct {
	client  *http.Client
	baseURL *url.URL
	config  map[string]string
	logger  hclog.Logger
}

// NewInfluxDBPlugin returns a new InfluxDB APM plugin instance.
func NewInfluxDBPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

// SetConfig parses and validates the plugin configuration. It mirrors the
// Datadog plugin's early-validation approach: all required fields are checked
// before any client is stored.
func (a *APMPlugin) SetConfig(config map[string]string) error {
	a.config = config

	address := strings.TrimSpace(a.config[configKeyAddress])
	if address == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	database := a.database()
	if database == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyDatabase)
	}

	version := strings.TrimSpace(a.config[configKeyVersion])
	if version == "" {
		version = configVersion1
	}
	if version != configVersion1 {
		switch version {
		case "2", "3":
			return fmt.Errorf("influxdb version %q is not yet supported: only version %q is currently implemented", version, configVersion1)
		default:
			return fmt.Errorf("invalid influxdb version %q: only version %q is supported", version, configVersion1)
		}
	}

	parsedURL, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %v", configKeyAddress, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%q must be a valid absolute URL", configKeyAddress)
	}

	a.baseURL = parsedURL
	a.client = &http.Client{}

	return nil
}

// PluginInfo returns metadata about the plugin.
func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Query executes a single InfluxQL query and returns exactly one metric
// stream. If the query returns zero or more than one series, an appropriate
// error is returned. This matches the Datadog plugin's Query semantics.
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
func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	a.logger.Debug("querying InfluxDB", "query", q, "range", r)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryURL, err := url.Parse(a.baseURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to construct query URL: %v", err)
	}
	queryURL.Path = strings.TrimSuffix(queryURL.Path, "/") + "/query"

	queryValues := queryURL.Query()
	queryValues.Set("db", a.database())
	queryValues.Set("q", q)
	queryValues.Set("epoch", "s")

	if user := strings.TrimSpace(a.config[configKeyUsername]); user != "" {
		queryValues.Set("u", user)
	}
	if pass := strings.TrimSpace(a.config[configKeyPassword]); pass != "" {
		queryValues.Set("p", pass)
	}
	queryURL.RawQuery = queryValues.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build influxdb query request: %v", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error querying metrics from influxdb: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("influxdb query failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var queryResp influxQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode influxdb query response: %v", err)
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

// database returns the configured database name, checking the primary key
// first and falling back to the shorthand alias.
func (a *APMPlugin) database() string {
	if db := strings.TrimSpace(a.config[configKeyDatabase]); db != "" {
		return db
	}
	return strings.TrimSpace(a.config[configKeyDB])
}

// parseSeries converts an InfluxDB result series into sdk.TimestampedMetrics.
// It locates the "time" column and picks the first non-time column as the
// metric value (preferring one literally named "value").
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
