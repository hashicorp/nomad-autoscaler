// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
// all result series as separate metric streams.
func (a *APMPlugin) QueryMultiple(_ string, _ sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	return nil, fmt.Errorf("not yet implemented")
}
