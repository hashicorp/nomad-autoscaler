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
			pm := NewPluginManager(logger, tc.pluginDir, tc.cfg)
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
			pm := NewPluginManager(logger, tc.pluginDir, tc.cfg)
			defer pm.KillPlugins()

			err := pm.Load()
			assert.NoError(t, err)

			for pluginType, plugins := range tc.cfg {
				for _, pluginConfig := range plugins {
					p, err := pm.dispense(pluginConfig.Name, pluginType)
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
