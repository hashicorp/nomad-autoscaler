// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	logger := hclog.NewNullLogger()

	cases := []struct {
		name        string
		pluginDir   string
		cfg         map[string][]*config.Plugin
		expectError bool
	}{
		{
			name:      "internal plugin",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"apm": {
					&config.Plugin{
						Name:   "nomad",
						Driver: "nomad-apm",
					},
					&config.Plugin{
						Name:   "prometheus",
						Driver: "prometheus",
						Config: map[string]string{
							"address": "http://example.com",
						},
					},
				},
				"target": {
					&config.Plugin{
						Name:   "nomad",
						Driver: "nomad-target",
					},
				},
				"strategy": {
					&config.Plugin{
						Name:   "target-value",
						Driver: "target-value",
					},
				},
			},
			expectError: false,
		},
		{
			name:      "external plugin",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"strategy": {
					&config.Plugin{
						Name:   "noop",
						Driver: "noop-strategy",
					},
				},
			},
			expectError: false,
		},
		{
			name:      "plugin doesnt exist",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"strategy": {
					&config.Plugin{
						Name:   "noop",
						Driver: "invalid-binary",
					},
				},
			},
			expectError: true,
		},
		{
			name:      "plugin doesnt exist",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"strategy": {
					&config.Plugin{
						Name:   "noop",
						Driver: "noop-fake-strategy",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm := newPluginManager(logger, tc.pluginDir, tc.cfg)
			err := pm.Load()
			defer pm.KillPlugins()

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDispense(t *testing.T) {
	logger := hclog.NewNullLogger()

	cases := []struct {
		name      string
		pluginDir string
		cfg       map[string][]*config.Plugin
	}{
		{
			name:      "internal plugin",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"apm": {
					&config.Plugin{
						Name:   "nomad",
						Driver: "nomad-apm",
					},
					&config.Plugin{
						Name:   "prometheus",
						Driver: "prometheus",
						Config: map[string]string{
							"address": "http://example.com",
						},
					},
				},
				"target": {
					&config.Plugin{
						Name:   "nomad",
						Driver: "nomad-target",
					},
				},
				"strategy": {
					&config.Plugin{
						Name:   "target-value",
						Driver: "target-value",
					},
				},
			},
		},
		{
			name:      "external plugin",
			pluginDir: "../test/bin",
			cfg: map[string][]*config.Plugin{
				"apm": {
					&config.Plugin{
						Name:   "noop",
						Driver: "noop-apm",
					},
				},
				"strategy": {
					&config.Plugin{
						Name:   "noop",
						Driver: "noop-strategy",
					},
				},
				"target": {
					&config.Plugin{
						Name:   "noop",
						Driver: "noop-target",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm := newPluginManager(logger, tc.pluginDir, tc.cfg)
			defer pm.KillPlugins()

			err := pm.Load()
			assert.NoError(t, err)

			for pluginType, plugins := range tc.cfg {
				for _, pluginConfig := range plugins {
					p, err := pm.Dispense(pluginConfig.Name, pluginType)
					assert.NotNil(t, p)
					assert.NoError(t, err)

					var inst interface{}
					switch pluginType {
					case "apm":
						inst = p.Plugin().(apm.APM)
					case "strategy":
						inst = p.Plugin().(strategy.Strategy)
					case "target":
						inst = p.Plugin().(target.Target)
					}
					assert.NotNil(t, inst)
				}
			}
		})
	}
}

func Test_setupPluginConfig(t *testing.T) {
	testCases := []struct {
		inputCfg map[string]string
		//inputAgent        *Agent
		inputNomadConfig  *api.Config
		expectedOutputCfg map[string]string
		name              string
	}{
		{
			inputCfg: map[string]string{
				"nomad_config_inherit": "false",
			},
			inputNomadConfig: &api.Config{
				Address:   "test",
				Region:    "test",
				SecretID:  "test",
				Namespace: "test",
				HttpAuth: &api.HttpBasicAuth{
					Username: "test",
					Password: "test",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "test",
					CAPath:        "test",
					ClientCert:    "test",
					ClientKey:     "test",
					TLSServerName: "test",
					Insecure:      true,
				},
			},

			expectedOutputCfg: map[string]string{
				"nomad_config_inherit": "false",
			},
			name: "Nomad config merging disabled ",
		},
		{
			inputCfg: map[string]string{
				"nomad_config_inherit": "falso",
			},
			inputNomadConfig: &api.Config{
				Address:   "test",
				Region:    "test",
				SecretID:  "test",
				Namespace: "test",
				HttpAuth: &api.HttpBasicAuth{
					Username: "test",
					Password: "test",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "test",
					CAPath:        "test",
					ClientCert:    "test",
					ClientKey:     "test",
					TLSServerName: "test",
					Insecure:      true,
				},
			},

			expectedOutputCfg: map[string]string{
				"nomad_config_inherit": "falso",
			},
			name: "Nomad config merging key set but value not parsable",
		},
		{
			inputCfg: map[string]string{
				"nomad_config_inherit": "true",
			},

			inputNomadConfig: &api.Config{
				Address:   "test",
				Region:    "test",
				SecretID:  "test",
				Namespace: "test",
				HttpAuth: &api.HttpBasicAuth{
					Username: "test",
					Password: "test",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "test",
					CAPath:        "test",
					ClientCert:    "test",
					ClientKey:     "test",
					TLSServerName: "test",
					Insecure:      true,
				},
			},
			expectedOutputCfg: map[string]string{
				"nomad_config_inherit":  "true",
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test:test",
				"nomad_ca-cert":         "test",
				"nomad_ca-path":         "test",
				"nomad_client-cert":     "test",
				"nomad_client-key":      "test",
				"nomad_tls-server-name": "test",
				"nomad_skip-verify":     "true",
			},
			name: "Nomad config merging key set to true",
		},
		{
			inputCfg: map[string]string{},
			inputNomadConfig: &api.Config{
				Address:   "test",
				Region:    "test",
				SecretID:  "test",
				Namespace: "test",
				HttpAuth: &api.HttpBasicAuth{
					Username: "test",
					Password: "test",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "test",
					CAPath:        "test",
					ClientCert:    "test",
					ClientKey:     "test",
					TLSServerName: "test",
					Insecure:      true,
				},
			},
			expectedOutputCfg: map[string]string{
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test:test",
				"nomad_ca-cert":         "test",
				"nomad_ca-path":         "test",
				"nomad_client-cert":     "test",
				"nomad_client-key":      "test",
				"nomad_tls-server-name": "test",
				"nomad_skip-verify":     "true",
			},
			name: "Nomad config merging key not set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupPluginConfig(tc.inputNomadConfig, tc.inputCfg)
			assert.Equal(t, tc.expectedOutputCfg, tc.inputCfg, tc.name)
		})
	}
}
