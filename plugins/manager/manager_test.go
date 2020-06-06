package manager

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
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
				"apm": []*config.Plugin{
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
				"target": []*config.Plugin{
					&config.Plugin{
						Name:   "nomad",
						Driver: "nomad-target",
					},
				},
				"strategy": []*config.Plugin{
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
				"strategy": []*config.Plugin{
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
				"strategy": []*config.Plugin{
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
				"strategy": []*config.Plugin{
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

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
