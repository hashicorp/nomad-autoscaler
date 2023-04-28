// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/api"
)

// pluginRoundTripper is used to configure the Prometheus HTTP client.
type pluginRoundTripper struct {
	headers           map[string]string
	basicAuthUser     string
	basicAuthPassword string

	rt http.RoundTripper
}

// newPluginRoudTripper returns a new pluginRoundTripper configured based on
// configuration values set for the plugin.
func newPluginRoudTripper(config map[string]string, tlsConfig *tls.Config) *pluginRoundTripper {
	username := config[configKeyBasicAuthUser]
	password := config[configKeyBasicAuthPassword]

	headers := make(map[string]string)
	for k, v := range config {
		if strings.HasPrefix(k, configKeyHeadersPrefix) {
			header := strings.TrimPrefix(k, configKeyHeadersPrefix)
			headers[header] = v
		}
	}

	defaultRoudTripper := api.DefaultRoundTripper.(*http.Transport)
	defaultRoudTripper.TLSClientConfig = tlsConfig

	return &pluginRoundTripper{
		headers:           headers,
		basicAuthUser:     username,
		basicAuthPassword: password,
		rt:                defaultRoudTripper,
	}
}

func (rt *pluginRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for header, value := range rt.headers {
		req.Header.Add(header, value)
	}

	setAuth := (rt.basicAuthUser != "" || rt.basicAuthPassword != "") && req.Header.Get("Authorization") == ""
	if setAuth {
		req.SetBasicAuth(rt.basicAuthUser, rt.basicAuthPassword)
	}

	return rt.rt.RoundTrip(req)
}
