package agent

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/stretchr/testify/assert"
)

func TestAgent_setupPluginConfig(t *testing.T) {
	testCases := []struct {
		inputCfg          map[string]string
		inputAgent        *Agent
		expectedOutputCfg map[string]string
		name              string
	}{
		{
			inputCfg: map[string]string{
				"nomad_config_inherit": "false",
			},
			inputAgent: &Agent{
				logger: hclog.NewNullLogger(),
				config: &config.Agent{
					Nomad: &config.Nomad{
						Address:       "test",
						Region:        "test",
						Namespace:     "test",
						Token:         "test",
						HTTPAuth:      "test",
						CACert:        "test",
						CAPath:        "test",
						ClientCert:    "test",
						ClientKey:     "test",
						TLSServerName: "test",
						SkipVerify:    true,
					},
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
			inputAgent: &Agent{
				logger: hclog.NewNullLogger(),
				config: &config.Agent{
					Nomad: &config.Nomad{
						Address:       "test",
						Region:        "test",
						Namespace:     "test",
						Token:         "test",
						HTTPAuth:      "test",
						CACert:        "test",
						CAPath:        "test",
						ClientCert:    "test",
						ClientKey:     "test",
						TLSServerName: "test",
						SkipVerify:    true,
					},
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
			inputAgent: &Agent{
				logger: hclog.NewNullLogger(),
				config: &config.Agent{
					Nomad: &config.Nomad{
						Address:       "test",
						Region:        "test",
						Namespace:     "test",
						Token:         "test",
						HTTPAuth:      "test",
						CACert:        "test",
						CAPath:        "test",
						ClientCert:    "test",
						ClientKey:     "test",
						TLSServerName: "test",
						SkipVerify:    true,
					},
				},
			},
			expectedOutputCfg: map[string]string{
				"nomad_config_inherit":  "true",
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test",
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
			inputAgent: &Agent{
				logger: hclog.NewNullLogger(),
				config: &config.Agent{
					Nomad: &config.Nomad{
						Address:       "test",
						Region:        "test",
						Namespace:     "test",
						Token:         "test",
						HTTPAuth:      "test",
						CACert:        "test",
						CAPath:        "test",
						ClientCert:    "test",
						ClientKey:     "test",
						TLSServerName: "test",
						SkipVerify:    true,
					},
				},
			},
			expectedOutputCfg: map[string]string{
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test",
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
			tc.inputAgent.setupPluginConfig(tc.inputCfg)
			assert.Equal(t, tc.expectedOutputCfg, tc.inputCfg, tc.name)
		})
	}
}
