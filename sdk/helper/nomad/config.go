package nomad

import (
	"strconv"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
)

const (
	configKeyNomadAddress       = "nomad_address"
	configKeyNomadRegion        = "nomad_region"
	configKeyNomadNamespace     = "nomad_namespace"
	configKeyNomadToken         = "nomad_token"
	configKeyNomadHTTPAuth      = "nomad_http-auth"
	configKeyNomadCACert        = "nomad_ca-cert"
	configKeyNomadCAPath        = "nomad_ca-path"
	configKeyNomadClientCert    = "nomad_client-cert"
	configKeyNomadClientKey     = "nomad_client-key"
	configKeyNomadTLSServerName = "nomad_tls-server-name"
	configKeyNomadSkipVerify    = "nomad_skip-verify"
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

// MergeMapWithAgentConfig merges a Nomad map config with an agent config. The
// config parameters within the map take preference.
func MergeMapWithAgentConfig(m map[string]string, agentCfg *config.Nomad) {
	if agentCfg == nil {
		return
	}

	if agentCfg.Address != "" && m[configKeyNomadAddress] == "" {
		m[configKeyNomadAddress] = agentCfg.Address
	}
	if agentCfg.Region != "" && m[configKeyNomadRegion] == "" {
		m[configKeyNomadRegion] = agentCfg.Region
	}
	if agentCfg.Namespace != "" && m[configKeyNomadNamespace] == "" {
		m[configKeyNomadNamespace] = agentCfg.Namespace
	}
	if agentCfg.Token != "" && m[configKeyNomadToken] == "" {
		m[configKeyNomadToken] = agentCfg.Token
	}
	if agentCfg.CACert != "" && m[configKeyNomadCACert] == "" {
		m[configKeyNomadCACert] = agentCfg.CACert
	}
	if agentCfg.CAPath != "" && m[configKeyNomadCAPath] == "" {
		m[configKeyNomadCAPath] = agentCfg.CAPath
	}
	if agentCfg.ClientCert != "" && m[configKeyNomadClientCert] == "" {
		m[configKeyNomadClientCert] = agentCfg.ClientCert
	}
	if agentCfg.ClientKey != "" && m[configKeyNomadClientKey] == "" {
		m[configKeyNomadClientKey] = agentCfg.ClientKey
	}
	if agentCfg.TLSServerName != "" && m[configKeyNomadTLSServerName] == "" {
		m[configKeyNomadTLSServerName] = agentCfg.TLSServerName
	}
	if agentCfg.SkipVerify && m[configKeyNomadSkipVerify] == "" {
		m[configKeyNomadSkipVerify] = strconv.FormatBool(agentCfg.SkipVerify)
	}
	if agentCfg.HTTPAuth != "" && m[configKeyNomadHTTPAuth] == "" {
		m[configKeyNomadHTTPAuth] = agentCfg.HTTPAuth
	}
}
