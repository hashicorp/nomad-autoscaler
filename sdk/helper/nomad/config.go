// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
)

const (
	configKeyNomadAddress            = "nomad_address"
	configKeyNomadRegion             = "nomad_region"
	configKeyNomadNamespace          = "nomad_namespace"
	configKeyNomadToken              = "nomad_token"
	configKeyNomadHTTPAuth           = "nomad_http-auth"
	configKeyNomadCACert             = "nomad_ca-cert"
	configKeyNomadCAPath             = "nomad_ca-path"
	configKeyNomadClientCert         = "nomad_client-cert"
	configKeyNomadClientKey          = "nomad_client-key"
	configKeyNomadTLSServerName      = "nomad_tls-server-name"
	configKeyNomadSkipVerify         = "nomad_skip-verify"
	configKeyNomadBlockQueryWaitTime = "nomad_block-query-wait-time"
)

// ConfigFromNamespacedMap converts the map representation of a Nomad config to
// the proper object that can be used to setup a client.
func ConfigFromNamespacedMap(cfg map[string]string) *api.Config {
	c := &api.Config{
		TLSConfig: &api.TLSConfig{},
	}

	if addr, ok := cfg[configKeyNomadAddress]; ok {
		c.Address = addr
	}
	if region, ok := cfg[configKeyNomadRegion]; ok {
		c.Region = region
	}
	if namespace, ok := cfg[configKeyNomadNamespace]; ok {
		c.Namespace = namespace
	}
	if token, ok := cfg[configKeyNomadToken]; ok {
		c.SecretID = token
	}
	if caCert, ok := cfg[configKeyNomadCACert]; ok {
		c.TLSConfig.CACert = caCert
	}
	if caPath, ok := cfg[configKeyNomadCAPath]; ok {
		c.TLSConfig.CAPath = caPath
	}
	if clientCert, ok := cfg[configKeyNomadClientCert]; ok {
		c.TLSConfig.ClientCert = clientCert
	}
	if clientKey, ok := cfg[configKeyNomadClientKey]; ok {
		c.TLSConfig.ClientKey = clientKey
	}
	if serverName, ok := cfg[configKeyNomadTLSServerName]; ok {
		c.TLSConfig.TLSServerName = serverName
	}
	// It should be safe to ignore any error when converting the string to a
	// bool. The boolean value should only ever come from a bool-flag, and
	// therefore we shouldn't have any risk of incorrect or malformed user
	// input string data.
	if skipVerify, ok := cfg[configKeyNomadSkipVerify]; ok {
		c.TLSConfig.Insecure, _ = strconv.ParseBool(skipVerify)
	}
	if httpAuth, ok := cfg[configKeyNomadHTTPAuth]; ok {
		c.HttpAuth = HTTPAuthFromString(httpAuth)
	}
	// We may ignore errors from ParseDuration() here as the provided argument
	// has already passed general validation as part of being a DurationVar flag.
	if blockQueryWaitTime, ok := cfg[configKeyNomadBlockQueryWaitTime]; ok {
		c.WaitTime, _ = time.ParseDuration(blockQueryWaitTime)
	}

	return c
}

// HTTPAuthFromString take an input string, and converts this to a Nomad API
// representation of basic HTTP auth.
func HTTPAuthFromString(auth string) *api.HttpBasicAuth {
	if auth == "" {
		return nil
	}

	var username, password string
	if strings.Contains(auth, ":") {
		split := strings.SplitN(auth, ":", 2)
		username = split[0]
		password = split[1]
	} else {
		username = auth
	}

	return &api.HttpBasicAuth{
		Username: username,
		Password: password,
	}
}

// MergeMapWithAgentConfig merges a Nomad map config with an API config object
// with the map config taking precedence. This allows users to override only a
// subset of params, while inheriting the agent configured items which are also
// derived from Nomad API default and env vars.
func MergeMapWithAgentConfig(m map[string]string, cfg *api.Config) {
	if cfg == nil {
		return
	}

	if cfg.Address != "" && m[configKeyNomadAddress] == "" {
		if cfg.URL() != nil {
			m[configKeyNomadAddress] = cfg.URL().String()
		} else {
			m[configKeyNomadAddress] = cfg.Address
		}
	}
	if cfg.Region != "" && m[configKeyNomadRegion] == "" {
		m[configKeyNomadRegion] = cfg.Region
	}
	if cfg.Namespace != "" && m[configKeyNomadNamespace] == "" {
		m[configKeyNomadNamespace] = cfg.Namespace
	}
	if cfg.SecretID != "" && m[configKeyNomadToken] == "" {
		m[configKeyNomadToken] = cfg.SecretID
	}
	if cfg.TLSConfig.CACert != "" && m[configKeyNomadCACert] == "" {
		m[configKeyNomadCACert] = cfg.TLSConfig.CACert
	}
	if cfg.TLSConfig.CAPath != "" && m[configKeyNomadCAPath] == "" {
		m[configKeyNomadCAPath] = cfg.TLSConfig.CAPath
	}
	if cfg.TLSConfig.ClientCert != "" && m[configKeyNomadClientCert] == "" {
		m[configKeyNomadClientCert] = cfg.TLSConfig.ClientCert
	}
	if cfg.TLSConfig.ClientKey != "" && m[configKeyNomadClientKey] == "" {
		m[configKeyNomadClientKey] = cfg.TLSConfig.ClientKey
	}
	if cfg.TLSConfig.TLSServerName != "" && m[configKeyNomadTLSServerName] == "" {
		m[configKeyNomadTLSServerName] = cfg.TLSConfig.TLSServerName
	}
	if cfg.TLSConfig.Insecure && m[configKeyNomadSkipVerify] == "" {
		m[configKeyNomadSkipVerify] = strconv.FormatBool(cfg.TLSConfig.Insecure)
	}
	if cfg.HttpAuth != nil && m[configKeyNomadHTTPAuth] == "" {
		auth := cfg.HttpAuth.Username
		if cfg.HttpAuth.Password != "" {
			auth += ":" + cfg.HttpAuth.Password
		}
		m[configKeyNomadHTTPAuth] = auth
	}
	if cfg.WaitTime != 0 && m[configKeyNomadBlockQueryWaitTime] == "" {
		m[configKeyNomadBlockQueryWaitTime] = cfg.WaitTime.String()
	}
}

// MergeDefaultWithAgentConfig merges the agent Nomad configuration with the
// default Nomad API configuration. The Nomad Autoscaler agent config takes
// precedence over the default config as any user supplied variables should
// override those configured by default or discovered via env vars within the
// Nomad API config.
func MergeDefaultWithAgentConfig(agentCfg *config.Nomad) *api.Config {

	// Use the Nomad API default config which gets populated by defaults and
	// also checks for environment variables.
	cfg := api.DefaultConfig()

	// Merge our top level configuration options in.
	if agentCfg.Address != "" {
		cfg.Address = agentCfg.Address
	}
	if agentCfg.Region != "" {
		cfg.Region = agentCfg.Region
	}
	if agentCfg.Namespace != "" {
		cfg.Namespace = agentCfg.Namespace
	}
	if agentCfg.Token != "" {
		cfg.SecretID = agentCfg.Token
	}

	// Merge HTTP auth.
	if agentCfg.HTTPAuth != "" {
		cfg.HttpAuth = HTTPAuthFromString(agentCfg.HTTPAuth)
	}

	// Merge TLS. The default config has an empty TLS object and therefore does
	// not required a nil check.
	if agentCfg.CACert != "" {
		cfg.TLSConfig.CACert = agentCfg.CACert
	}
	if agentCfg.CAPath != "" {
		cfg.TLSConfig.CAPath = agentCfg.CAPath
	}
	if agentCfg.ClientCert != "" {
		cfg.TLSConfig.ClientCert = agentCfg.ClientCert
	}
	if agentCfg.ClientKey != "" {
		cfg.TLSConfig.ClientKey = agentCfg.ClientKey
	}
	if agentCfg.TLSServerName != "" {
		cfg.TLSConfig.TLSServerName = agentCfg.TLSServerName
	}
	if agentCfg.SkipVerify {
		cfg.TLSConfig.Insecure = agentCfg.SkipVerify
	}
	if agentCfg.BlockQueryWaitTime != 0 {
		cfg.WaitTime = agentCfg.BlockQueryWaitTime
	}

	return cfg
}
