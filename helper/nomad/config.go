package nomad

import (
	"strconv"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
)

// configKeys are the map key values used when creating a config map based off
// an internal Nomad configuration representation.
const (
	configKeyAddress       = "address"
	configKeyRegion        = "region"
	configKeyNamespace     = "namespace"
	configKeyToken         = "token"
	configKeyHTTPAuth      = "http-auth"
	configKeyCACert        = "ca-cert"
	configKeyCAPath        = "ca-path"
	configKeyClientCert    = "client-cert"
	configKeyClientKey     = "client-key"
	configKeyTLSServerName = "tls-server-name"
	configKeySkipVerify    = "skip-verify"
)

// ConfigToMap takes the Autoscalers internal configuration for Nomad API
// connectivity and translates this to a map. This helper is needed when
// passing configuration to plugins via generic structures.
func ConfigToMap(cfg *config.Nomad) map[string]string {
	m := make(map[string]string)

	if cfg.Address != "" {
		m[configKeyAddress] = cfg.Address
	}
	if cfg.Region != "" {
		m[configKeyRegion] = cfg.Region
	}
	if cfg.Namespace != "" {
		m[configKeyNamespace] = cfg.Namespace
	}
	if cfg.Token != "" {
		m[configKeyToken] = cfg.Token
	}
	if cfg.CACert != "" {
		m[configKeyCACert] = cfg.CACert
	}
	if cfg.CAPath != "" {
		m[configKeyCAPath] = cfg.CAPath
	}
	if cfg.ClientCert != "" {
		m[configKeyClientCert] = cfg.ClientCert
	}
	if cfg.ClientKey != "" {
		m[configKeyClientKey] = cfg.ClientKey
	}
	if cfg.TLSServerName != "" {
		m[configKeyTLSServerName] = cfg.TLSServerName
	}
	if cfg.SkipVerify {
		m[configKeySkipVerify] = strconv.FormatBool(cfg.SkipVerify)
	}
	if cfg.HTTPAuth != "" {
		m[configKeyHTTPAuth] = cfg.HTTPAuth
	}

	return m
}

// ConfigFromMap takes a generic map and converts it to a Nomad API config
// struct.
func ConfigFromMap(cfg map[string]string) *api.Config {
	c := &api.Config{
		TLSConfig: &api.TLSConfig{},
	}

	if addr, ok := cfg[configKeyAddress]; ok {
		c.Address = addr
	}
	if region, ok := cfg[configKeyRegion]; ok {
		c.Region = region
	}
	if namespace, ok := cfg[configKeyNamespace]; ok {
		c.Namespace = namespace
	}
	if token, ok := cfg[configKeyToken]; ok {
		c.SecretID = token
	}
	if caCert, ok := cfg[configKeyCACert]; ok {
		c.TLSConfig.CACert = caCert
	}
	if caPath, ok := cfg[configKeyCAPath]; ok {
		c.TLSConfig.CAPath = caPath
	}
	if clientCert, ok := cfg[configKeyClientCert]; ok {
		c.TLSConfig.ClientCert = clientCert
	}
	if clientKey, ok := cfg[configKeyClientKey]; ok {
		c.TLSConfig.ClientKey = clientKey
	}
	if serverName, ok := cfg[configKeyTLSServerName]; ok {
		c.TLSConfig.TLSServerName = serverName
	}
	// It should be safe to ignore any error when converting the string to a
	// bool. The boolean value should only ever come from a bool-flag, and
	// therefore we shouldn't have any risk of incorrect or malformed user
	// input string data.
	if skipVerify, ok := cfg[configKeySkipVerify]; ok {
		c.TLSConfig.Insecure, _ = strconv.ParseBool(skipVerify)
	}
	if httpAuth, ok := cfg[configKeyHTTPAuth]; ok {
		c.HttpAuth = HTTPAuthFromString(httpAuth)
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
