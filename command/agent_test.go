package command

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
)

func TestCommandAgent_readConfig(t *testing.T) {
	defaultConfig, _ := config.Default()
	defaultConfig = defaultConfig.Merge(config.DefaultEntConfig())

	testCases := []struct {
		name string
		args []string
		want *config.Agent
	}{
		{
			name: "no args",
			want: defaultConfig,
		},
		{
			name: "top level flags",
			args: []string{
				"-log-level", "WARN",
				"-log-json",
				"-enable-debug",
				"-disable-nomad-source",
				"-plugin-dir", "./plugins",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel:           "WARN",
				LogJson:            true,
				EnableDebug:        true,
				DisableNomadSource: true,
				PluginDir:          "./plugins",
			}),
		},
		{
			name: "http flags",
			args: []string{
				"-http-bind-address", "10.0.0.1",
				"-http-bind-port", "9999",
			},
			want: defaultConfig.Merge(&config.Agent{
				HTTP: &config.HTTP{
					BindAddress: "10.0.0.1",
					BindPort:    9999,
				},
			}),
		},
		{
			name: "nomad flags",
			args: []string{
				"-nomad-address", "http://nomad.example.com",
				"-nomad-region", "milky_way",
				"-nomad-namespace", "prod",
				"-nomad-token", "TOKEN",
				"-nomad-http-auth", "user:pass",
				"-nomad-ca-cert", "./ca-cert.pem",
				"-nomad-ca-path", "./ca-certs",
				"-nomad-client-cert", "./client-cert.pem",
				"-nomad-client-key", "./client-key.pem",
				"-nomad-tls-server-name", "server",
				"-nomad-skip-verify",
			},
			want: defaultConfig.Merge(&config.Agent{
				Nomad: &config.Nomad{
					Address:       "http://nomad.example.com",
					Region:        "milky_way",
					Namespace:     "prod",
					Token:         "TOKEN",
					HTTPAuth:      "user:pass",
					CACert:        "./ca-cert.pem",
					CAPath:        "./ca-certs",
					ClientCert:    "./client-cert.pem",
					ClientKey:     "./client-key.pem",
					TLSServerName: "server",
					SkipVerify:    true,
				},
			}),
		},
		{
			name: "policy flags",
			args: []string{
				"-policy-dir", "./policies",
				"-policy-default-cooldown", "10m",
				"-policy-default-evaluation-interval", "20s",
			},
			want: defaultConfig.Merge(&config.Agent{
				Policy: &config.Policy{
					Dir:                       "./policies",
					DefaultCooldown:           10 * time.Minute,
					DefaultEvaluationInterval: 20 * time.Second,
				},
			}),
		},
		{
			name: "policy eval flags",
			args: []string{
				"-policy-eval-ack-timeout", "30m",
				"-policy-eval-delivery-limit", "10",
				"-policy-eval-workers", "horizontal:1,cluster:2",
			},
			want: defaultConfig.Merge(&config.Agent{
				PolicyEval: &config.PolicyEval{
					DeliveryLimit: 10,
					AckTimeout:    30 * time.Minute,
					Workers: map[string]int{
						"horizontal": 1,
						"cluster":    2,
					},
				},
			}),
		},
		{
			name: "telemetry flags",
			args: []string{
				"-telemetry-disable-hostname",
				"-telemetry-enable-hostname-label",
				"-telemetry-collection-interval", "30s",
				"-telemetry-statsite-address", "statsite.example.com",
				"-telemetry-statsd-address", "statsd.example.com",
				"-telemetry-dogstatsd-address", "dogstatsd.example.com",
				"-telemetry-dogstatsd-tags", "my_tag_name1:my_tag_value1",
				"-telemetry-dogstatsd-tags", "my_tag_name2:my_tag_value2",
				"-telemetry-prometheus-metrics",
				"-telemetry-prometheus-retention-time", "14s",
				"-telemetry-circonus-api-token", "TOKEN",
				"-telemetry-circonus-api-app", "APP",
				"-telemetry-circonus-api-url", "http://circonus.example.com",
				"-telemetry-circonus-submission-interval", "50m",
				"-telemetry-circonus-submission-url", "http://circonus-sub.example.com",
				"-telemetry-circonus-check-id", "CHECK_ID",
				"-telemetry-circonus-check-force-metric-activation", "true",
				"-telemetry-circonus-check-instance-id", "CHECK_INSTANCE_ID",
				"-telemetry-circonus-check-search-tag", "SEARCH_TAG",
				"-telemetry-circonus-check-tags", "CHECK_TAGS",
				"-telemetry-circonus-check-display-name", "DISPLAY_NAME",
				"-telemetry-circonus-broker-id", "BROKER_ID",
				"-telemetry-circonus-broker-select-tag", "BROKER_SELECT_TAG",
			},
			want: defaultConfig.Merge(&config.Agent{
				Telemetry: &config.Telemetry{
					DisableHostname:                    true,
					EnableHostnameLabel:                true,
					CollectionInterval:                 30 * time.Second,
					StatsiteAddr:                       "statsite.example.com",
					StatsdAddr:                         "statsd.example.com",
					DogStatsDAddr:                      "dogstatsd.example.com",
					DogStatsDTags:                      []string{"my_tag_name1:my_tag_value1", "my_tag_name2:my_tag_value2"},
					PrometheusMetrics:                  true,
					PrometheusRetentionTime:            14 * time.Second,
					CirconusAPIToken:                   "TOKEN",
					CirconusAPIApp:                     "APP",
					CirconusAPIURL:                     "http://circonus.example.com",
					CirconusSubmissionInterval:         "50m",
					CirconusCheckSubmissionURL:         "http://circonus-sub.example.com",
					CirconusCheckID:                    "CHECK_ID",
					CirconusCheckForceMetricActivation: "true",
					CirconusCheckInstanceID:            "CHECK_INSTANCE_ID",
					CirconusCheckSearchTag:             "SEARCH_TAG",
					CirconusCheckTags:                  "CHECK_TAGS",
					CirconusCheckDisplayName:           "DISPLAY_NAME",
					CirconusBrokerID:                   "BROKER_ID",
					CirconusBrokerSelectTag:            "BROKER_SELECT_TAG",
				},
			}),
		},
		{
			name: "from file",
			args: []string{
				"-config", "./test-fixtures/agent_config_full.hcl",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel:           "TRACE",
				LogJson:            true,
				EnableDebug:        true,
				DisableNomadSource: true,
				PluginDir:          "./plugin_dir_from_file",
				HTTP: &config.HTTP{
					BindAddress: "10.0.0.2",
					BindPort:    8888,
				},
				Nomad: &config.Nomad{
					Address:       "http://nomad_from_file.example.com:4646",
					Region:        "file",
					Namespace:     "staging",
					Token:         "TOKEN_FROM_FILE",
					HTTPAuth:      "user:file",
					CACert:        "./ca-cert-from-file.pem",
					CAPath:        "./ca-cert-from-file",
					ClientCert:    "./client-cert-from-file.pem",
					ClientKey:     "./client-key-from-file.pem",
					TLSServerName: "tls_from_file",
					SkipVerify:    true,
				},
				Consul: &config.Consul{
					Addr:       "https://consul_from_file.example.com:8500",
					TimeoutHCL: "2m",
					Token:      "TOKEN_FROM_FILE",
					Auth:       "user:file",
					EnableSSL:  ptr.BoolToPtr(true),
					VerifySSL:  ptr.BoolToPtr(true),
					CAFile:     "./ca-from-file.pem",
					CertFile:   "./cert-from-file.pem",
					KeyFile:    "./key-from-file.pem",
					Namespace:  "namespace-from-file",
					Datacenter: "datacenter-from-file",
				},
				Policy: &config.Policy{
					Dir:                       "./policy-dir-from-file",
					DefaultCooldown:           12 * time.Second,
					DefaultEvaluationInterval: 50 * time.Minute,
				},
				PolicyEval: &config.PolicyEval{
					DeliveryLimit:    10,
					DeliveryLimitPtr: ptr.IntToPtr(10),
					AckTimeout:       3 * time.Minute,
					Workers: map[string]int{
						"cluster":    3,
						"horizontal": 1,
					},
				},
			}),
		},
		{
			name: "flags override files",
			args: []string{
				"-log-level", "TRACE",
				"-config", "./test-fixtures/agent_config_small.hcl",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel: "TRACE",
			}),
		},
		{
			name: "flags merge with files",
			args: []string{
				"-log-json",
				"-config", "./test-fixtures/agent_config_small.hcl",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel: "ERROR",
				LogJson:  true,
			}),
		},
		{
			name: "multiple files are merged",
			args: []string{
				"-config", "./test-fixtures/agent_config_small.hcl",
				"-config", "./test-fixtures/agent_config_small_2.hcl",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel: "ERROR",
				LogJson:  true,
			}),
		},
		{
			name: "load directory",
			args: []string{
				"-config", "./test-fixtures",
			},
			want: defaultConfig.Merge(&config.Agent{
				LogLevel:           "ERROR",
				LogJson:            true,
				EnableDebug:        true,
				DisableNomadSource: true,
				PluginDir:          "./plugin_dir_from_file",
				HTTP: &config.HTTP{
					BindAddress: "10.0.0.2",
					BindPort:    8888,
				},
				Nomad: &config.Nomad{
					Address:       "http://nomad_from_file.example.com:4646",
					Region:        "file",
					Namespace:     "staging",
					Token:         "TOKEN_FROM_FILE",
					HTTPAuth:      "user:file",
					CACert:        "./ca-cert-from-file.pem",
					CAPath:        "./ca-cert-from-file",
					ClientCert:    "./client-cert-from-file.pem",
					ClientKey:     "./client-key-from-file.pem",
					TLSServerName: "tls_from_file",
					SkipVerify:    true,
				},
				Consul: &config.Consul{
					Addr:       "https://consul_from_file.example.com:8500",
					TimeoutHCL: "2m",
					Token:      "TOKEN_FROM_FILE",
					Auth:       "user:file",
					EnableSSL:  ptr.BoolToPtr(true),
					VerifySSL:  ptr.BoolToPtr(true),
					CAFile:     "./ca-from-file.pem",
					CertFile:   "./cert-from-file.pem",
					KeyFile:    "./key-from-file.pem",
					Namespace:  "namespace-from-file",
					Datacenter: "datacenter-from-file",
				},
				Policy: &config.Policy{
					Dir:                       "./policy-dir-from-file",
					DefaultCooldown:           12 * time.Second,
					DefaultEvaluationInterval: 50 * time.Minute,
				},
				PolicyEval: &config.PolicyEval{
					DeliveryLimit:    10,
					DeliveryLimitPtr: ptr.IntToPtr(10),
					AckTimeout:       3 * time.Minute,
					Workers: map[string]int{
						"cluster":    3,
						"horizontal": 1,
					},
				},
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &AgentCommand{args: tc.args}
			got, _ := c.readConfig()

			// Sort the list of plugins to avoid flakiness.
			sortOpt := cmpopts.SortSlices(func(x, y *config.Plugin) bool {
				return x.Name < y.Name
			})

			if diff := cmp.Diff(tc.want, got, sortOpt); diff != "" {
				t.Errorf("readConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
