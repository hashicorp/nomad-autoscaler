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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "influxdb"

	// configKeyAddress is the InfluxDB server address.
	// Falls back to INFLUXDB_ADDRESS if not set in the config map.
	configKeyAddress = "address"

	// configKeyDatabase is the primary config key for the database name.
	// Falls back to INFLUXDB_DATABASE if neither "database" nor "db" is set.
	configKeyDatabase = "database"

	// configKeyDB is the shorthand alias accepted for database name.
	// Lower priority than "database"; takes precedence over INFLUXDB_DATABASE.
	configKeyDB = "db"

	// configKeyUsername is the optional authentication username.
	// Required when shared_secret is set (used as the JWT "username" claim).
	// Falls back to INFLUXDB_USERNAME if not set in the config map.
	configKeyUsername = "username"

	// configKeyPassword is the optional authentication password.
	// Used together with username for HTTP Basic auth.
	// Cannot be used together with shared_secret.
	// Falls back to INFLUXDB_PASSWORD if not set in the config map.
	configKeyPassword = "password"

	// configKeySharedSecret is the InfluxDB shared secret used to sign
	// auto-generated HS256 JWTs. When set, the plugin generates and refreshes
	// Bearer JWTs internally — no manual token management required.
	// Requires: username. Conflicts with: password.
	// Corresponds to INFLUXDB_HTTP_SHARED_SECRET on the InfluxDB server.
	// Falls back to INFLUXDB_SHARED_SECRET if not set in the config map.
	configKeySharedSecret = "shared_secret"

	// configKeyTokenTTL is the optional lifetime for auto-generated JWTs.
	// Accepts Go duration strings (e.g. "30m", "2h"). Default: 1h.
	// Range: 1m – 24h. Only meaningful when shared_secret is set.
	configKeyTokenTTL = "token_ttl"

	// configKeyVersion selects the InfluxDB API version. Only "1" is
	// currently supported; "2" and "3" are reserved for future implementation.
	configKeyVersion = "version"

	// configVersion1 is the default and only supported version today.
	configVersion1 = "1"

	// defaultTokenTTL is the JWT lifetime when token_ttl is not configured.
	defaultTokenTTL = time.Hour

	// minTokenTTL / maxTokenTTL are the allowed bounds for token_ttl.
	minTokenTTL = time.Minute
	maxTokenTTL = 24 * time.Hour

	// queryTimeout is the per-request deadline for InfluxDB HTTP queries.
	queryTimeout = 10 * time.Second

	// Environment variable names for credential fallback.
	// The config map always takes precedence; these are checked only when the
	// corresponding config key is absent or empty.
	envVarAddress      = "INFLUXDB_ADDRESS"
	envVarDatabase     = "INFLUXDB_DATABASE"
	envVarUsername     = "INFLUXDB_USERNAME"
	envVarPassword     = "INFLUXDB_PASSWORD"
	envVarSharedSecret = "INFLUXDB_SHARED_SECRET"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) any { return NewInfluxDBPlugin(l) },
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

// influxQueryResult holds the series and optional error for a single
// InfluxQL statement within a query response.
type influxQueryResult struct {
	StatementID int                 `json:"statement_id"`
	Series      []influxQuerySeries `json:"series"`
	Error       string              `json:"error,omitempty"`
}

// influxQuerySeries holds the column names and row values for one named
// measurement series returned by an InfluxQL query.
type influxQuerySeries struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Values  [][]any  `json:"values"`
}

// APMPlugin is the InfluxDB implementation of the APM interface.
type APMPlugin struct {
	client      *http.Client
	baseURL     *url.URL
	config      map[string]string
	logger      hclog.Logger
	tokenTTL    time.Duration // parsed from configKeyTokenTTL; set during SetConfig
	jwtMu       sync.Mutex    // protects cachedToken and tokenExpiry
	cachedToken string        // most recently generated JWT; empty until first use
	tokenExpiry time.Time     // when cachedToken expires
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
	// Copy the config map so we can write resolved values back without
	// mutating the caller's map. All validation runs on this copy, and it
	// becomes a.config only after all checks pass.
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}

	address := configOrEnv(cfg, configKeyAddress, envVarAddress)
	if address == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	// Resolve database: "database" key → "db" alias → INFLUXDB_DATABASE env var.
	database := strings.TrimSpace(cfg[configKeyDatabase])
	if database == "" {
		database = strings.TrimSpace(cfg[configKeyDB])
	}
	if database == "" {
		if v := strings.TrimSpace(os.Getenv(envVarDatabase)); v != "" {
			database = v
		}
	}
	if database == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyDatabase)
	}

	// Validate auth configuration mutual exclusivity.
	username := configOrEnv(cfg, configKeyUsername, envVarUsername)
	password := configOrEnv(cfg, configKeyPassword, envVarPassword)
	sharedSecret := configOrEnv(cfg, configKeySharedSecret, envVarSharedSecret)

	if sharedSecret != "" {
		if username == "" {
			return fmt.Errorf("auth configuration error: %q requires %q (used as the JWT username claim)", configKeySharedSecret, configKeyUsername)
		}
		if password != "" {
			return fmt.Errorf("conflicting auth configuration: %q cannot be used together with %q", configKeySharedSecret, configKeyPassword)
		}
	}

	// Parse and validate token_ttl.
	ttl := defaultTokenTTL
	if raw := strings.TrimSpace(cfg[configKeyTokenTTL]); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid %q value %q: %v", configKeyTokenTTL, raw, err)
		}
		if parsed < minTokenTTL || parsed > maxTokenTTL {
			return fmt.Errorf("invalid %q value %q: must be between %s and %s", configKeyTokenTTL, raw, minTokenTTL, maxTokenTTL)
		}
		ttl = parsed
	}

	switch version := strings.TrimSpace(cfg[configKeyVersion]); version {
	case "", configVersion1:
		// ok — v1 is the default
	case "2", "3":
		return fmt.Errorf("influxdb version %q is not yet supported: only version %q is currently implemented", version, configVersion1)
	default:
		return fmt.Errorf("invalid influxdb version %q: only version %q is supported", version, configVersion1)
	}

	parsedURL, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %v", configKeyAddress, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%q must be a valid absolute URL", configKeyAddress)
	}

	// Write resolved values back so downstream methods (setAuthHeader,
	// generateJWT) can read them from a.config without needing env var awareness.
	cfg[configKeyAddress] = address
	cfg[configKeyDatabase] = database
	cfg[configKeyUsername] = username
	cfg[configKeyPassword] = password
	cfg[configKeySharedSecret] = sharedSecret

	// All validation passed — atomically update plugin state.
	a.config = cfg
	a.baseURL = parsedURL
	a.client = &http.Client{}
	a.tokenTTL = ttl

	// Reset cached JWT so any previously generated token is not reused after
	// a config change.
	a.jwtMu.Lock()
	a.cachedToken = ""
	a.tokenExpiry = time.Time{}
	a.jwtMu.Unlock()

	return nil
}

// PluginInfo returns the plugin name and type.
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

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	queryURL := *a.baseURL
	queryURL.Path = strings.TrimSuffix(queryURL.Path, "/") + "/query"

	queryValues := queryURL.Query()
	queryValues.Set("db", a.database())
	queryValues.Set("q", q)
	queryValues.Set("epoch", "s")
	queryURL.RawQuery = queryValues.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build influxdb query request: %v", err)
	}

	if err := a.setAuthHeader(req); err != nil {
		return nil, fmt.Errorf("failed to set auth header: %v", err)
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

	var results []sdk.TimestampedMetrics
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

// database returns the resolved database name from the plugin configuration.
func (a *APMPlugin) database() string {
	return strings.TrimSpace(a.config[configKeyDatabase])
}

// setAuthHeader applies the appropriate authentication method to the HTTP
// request based on the plugin configuration:
//   - shared_secret + username → auto-generated HS256 JWT Bearer token
//   - username + password      → HTTP Basic auth
//   - neither                  → unauthenticated (local/dev scenarios)
func (a *APMPlugin) setAuthHeader(req *http.Request) error {
	if sharedSecret := strings.TrimSpace(a.config[configKeySharedSecret]); sharedSecret != "" {
		token, err := a.getOrRefreshJWT()
		if err != nil {
			return fmt.Errorf("failed to generate JWT: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	username := strings.TrimSpace(a.config[configKeyUsername])
	password := strings.TrimSpace(a.config[configKeyPassword])
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	return nil
}

// getOrRefreshJWT returns the cached JWT if it is still fresh, or generates
// and caches a new one. The refresh window is max(30s, 10% of token_ttl),
// so a token is regenerated slightly before it would expire.
func (a *APMPlugin) getOrRefreshJWT() (string, error) {
	refreshWindow := max(a.tokenTTL/10, 30*time.Second)

	a.jwtMu.Lock()
	defer a.jwtMu.Unlock()

	if a.cachedToken != "" && time.Until(a.tokenExpiry) > refreshWindow {
		return a.cachedToken, nil
	}

	token, expiry, err := a.generateJWT()
	if err != nil {
		return "", err
	}

	a.cachedToken = token
	a.tokenExpiry = expiry
	a.logger.Debug("generated new InfluxDB JWT", "expires_at", expiry.UTC().Format(time.RFC3339))
	return token, nil
}

// generateJWT creates a new HS256-signed JWT for InfluxDB 1.x Bearer auth.
// The JWT payload contains the configured username and an expiry of now+tokenTTL,
// matching the format expected by InfluxDB's shared-secret JWT authentication.
func (a *APMPlugin) generateJWT() (string, time.Time, error) {
	expiry := time.Now().Add(a.tokenTTL)
	claims := jwtlib.MapClaims{
		"username": strings.TrimSpace(a.config[configKeyUsername]),
		"exp":      expiry.Unix(),
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(strings.TrimSpace(a.config[configKeySharedSecret])))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiry, nil
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

// parseEpochSeconds converts an any value (typically a JSON number) into an
// int64 Unix epoch in seconds.
func parseEpochSeconds(v any) (int64, error) {
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

// parseFloatValue converts an any value (typically a JSON number) into a
// float64 metric value.
func parseFloatValue(v any) (float64, error) {
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

// configOrEnv returns the trimmed value of config[key] if non-empty, otherwise
// the trimmed value of the environment variable envVar. Returns "" if neither
// is set. The config map always takes precedence over the environment variable.
func configOrEnv(config map[string]string, key, envVar string) string {
	if v := strings.TrimSpace(config[key]); v != "" {
		return v
	}
	v, _ := os.LookupEnv(envVar)
	return strings.TrimSpace(v)
}
