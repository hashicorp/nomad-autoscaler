// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Default(t *testing.T) {
	def, err := Default()
	assert.Nil(t, err)
	assert.NotNil(t, def)
	assert.False(t, def.LogJson)
	assert.Equal(t, def.LogLevel, "info")
	assert.True(t, strings.HasSuffix(def.PluginDir, "/plugins"))
	assert.Equal(t, def.Policy.DefaultEvaluationInterval, 10*time.Second)
	assert.Equal(t, "127.0.0.1", def.HTTP.BindAddress)
	assert.Equal(t, 8080, def.HTTP.BindPort)
	assert.Equal(t, def.Policy.DefaultCooldown, 5*time.Minute)
	assert.Len(t, def.Policy.Sources, 2)
	assert.Equal(t, defaultPolicyEvalDeliveryLimit, def.PolicyEval.DeliveryLimit)
	assert.Equal(t, defaultPolicyEvalAckTimeout, def.PolicyEval.AckTimeout)
	assert.Equal(t, defaultPolicyEvalWorkers, def.PolicyEval.Workers)
	assert.Len(t, def.APMs, 1)
	assert.Len(t, def.Targets, 1)
	assert.Len(t, def.Strategies, 4)
	assert.Equal(t, 1*time.Second, def.Telemetry.CollectionInterval)
	assert.False(t, def.EnableDebug, "ensure debugging is disabled by default")
}

func TestAgent_Merge(t *testing.T) {
	baseCfg, err := Default()
	assert.Nil(t, err)

	cfg1 := &Agent{
		PluginDir: "/opt/nomad-autoscaler/plugins",
		DynamicApplicationSizing: &DynamicApplicationSizing{
			MetricsPreloadThreshold: 24 * time.Hour,
		},
		HTTP: &HTTP{
			BindAddress: "scaler.nomad",
		},
		Nomad: &Nomad{
			Address: "http://nomad.systems:4646",
		},
		PolicyEval: &PolicyEval{
			Workers: map[string]int{
				"horizontal": 5,
				"some-other": 3,
			},
		},
		Policy: &Policy{
			Sources: []*PolicySource{
				{
					Name:    "nomad",
					Enabled: ptr.BoolToPtr(false),
				},
			},
		},
		APMs: []*Plugin{
			{
				Name:   "prometheus",
				Driver: "prometheus",
				Config: map[string]string{"address": "http://prometheus.systems:9090"},
			},
		},
	}

	cfg2 := &Agent{
		EnableDebug: true,
		LogLevel:    "trace",
		LogJson:     true,
		PluginDir:   "/var/lib/nomad-autoscaler/plugins",
		DynamicApplicationSizing: &DynamicApplicationSizing{
			MetricsPreloadThreshold: 12 * time.Hour,
			EvaluateAfter:           2 * time.Hour,
			NamespaceLabel:          "my_namespace",
			JobLabel:                "my_label",
			GroupLabel:              "my_group",
			TaskLabel:               "my_task",
			CPUMetric:               "custom_cpu_metric",
			MemoryMetric:            "custom_memory_metric",
		},
		HTTP: &HTTP{
			BindPort: 4646,
		},
		Consul: &Consul{},
		Nomad: &Nomad{
			Address:       "https://nomad-new.systems:4646",
			Region:        "moon-base-1",
			Namespace:     "fra-mauro",
			Token:         "super-secret-tokeny-thing",
			HTTPAuth:      "admin:admin",
			CACert:        "/etc/nomad.d/ca.crt",
			CAPath:        "/etc/nomad.d/ca/",
			ClientCert:    "/etc/nomad.d/client.crt",
			ClientKey:     "/etc/nomad.d/client-key.crt",
			TLSServerName: "cows-or-pets",
			SkipVerify:    true,
		},
		Policy: &Policy{
			Dir:                       "/etc/scaling/policies",
			DefaultCooldown:           20 * time.Minute,
			DefaultEvaluationInterval: 10 * time.Second,
			Sources: []*PolicySource{
				{
					Name:    "file",
					Enabled: ptr.BoolToPtr(false),
				},
				{
					Name:    "nomad",
					Enabled: ptr.BoolToPtr(true),
				},
			},
		},
		PolicyEval: &PolicyEval{
			DeliveryLimitPtr: ptr.IntToPtr(10),
			DeliveryLimit:    10,
			AckTimeout:       3 * time.Minute,
			Workers: map[string]int{
				"cluster":    8,
				"horizontal": 7,
			},
		},
		Telemetry: &Telemetry{
			StatsiteAddr:                       "some-address",
			StatsdAddr:                         "some-other-address",
			DogStatsDAddr:                      "some-other-other-address",
			DogStatsDTags:                      []string{"most-important-metric"},
			PrometheusMetrics:                  true,
			PrometheusRetentionTime:            48 * time.Hour,
			DisableHostname:                    true,
			CollectionInterval:                 3 * time.Second,
			CirconusAPIToken:                   "super-secret",
			CirconusAPIApp:                     "secret-app",
			CirconusAPIURL:                     "some-url",
			CirconusSubmissionInterval:         "30s",
			CirconusCheckSubmissionURL:         "some-other-url",
			CirconusCheckID:                    "who-knows",
			CirconusCheckForceMetricActivation: "true",
			CirconusCheckInstanceID:            "some-id",
			CirconusCheckSearchTag:             "some-tag",
			CirconusCheckTags:                  "some-tags",
			CirconusCheckDisplayName:           "some-name",
			CirconusBrokerID:                   "some-id",
			CirconusBrokerSelectTag:            "some-other-tag",
		},
		APMs: []*Plugin{
			{
				Name:   "influx-db",
				Driver: "influx-db",
			},
			{
				Name:   "prometheus",
				Driver: "prometheus",
				Config: map[string]string{"address": "http://prometheus-new.systems:9090"},
				Args:   []string{"all-the-encryption"},
			},
		},
		Strategies: []*Plugin{
			{
				Name:   "pid",
				Driver: "pid",
			},
		},
	}

	expectedResult := &Agent{
		EnableDebug: true,
		LogLevel:    "trace",
		LogJson:     true,
		PluginDir:   "/var/lib/nomad-autoscaler/plugins",
		DynamicApplicationSizing: &DynamicApplicationSizing{
			MetricsPreloadThreshold: 12 * time.Hour,
			EvaluateAfter:           2 * time.Hour,
			NamespaceLabel:          "my_namespace",
			JobLabel:                "my_label",
			GroupLabel:              "my_group",
			TaskLabel:               "my_task",
			CPUMetric:               "custom_cpu_metric",
			MemoryMetric:            "custom_memory_metric",
		},
		HTTP: &HTTP{
			BindAddress: "scaler.nomad",
			BindPort:    4646,
		},
		Consul: &Consul{},
		Nomad: &Nomad{
			Address:       "https://nomad-new.systems:4646",
			Region:        "moon-base-1",
			Namespace:     "fra-mauro",
			Token:         "super-secret-tokeny-thing",
			HTTPAuth:      "admin:admin",
			CACert:        "/etc/nomad.d/ca.crt",
			CAPath:        "/etc/nomad.d/ca/",
			ClientCert:    "/etc/nomad.d/client.crt",
			ClientKey:     "/etc/nomad.d/client-key.crt",
			TLSServerName: "cows-or-pets",
			SkipVerify:    true,
		},
		Policy: &Policy{
			Dir:                       "/etc/scaling/policies",
			DefaultCooldown:           20 * time.Minute,
			DefaultEvaluationInterval: 10 * time.Second,
			Sources: []*PolicySource{
				{
					Name:    "file",
					Enabled: ptr.BoolToPtr(false),
				},
				{
					Name:    "nomad",
					Enabled: ptr.BoolToPtr(true),
				},
			},
		},
		PolicyEval: &PolicyEval{
			DeliveryLimitPtr: ptr.IntToPtr(10),
			DeliveryLimit:    10,
			AckTimeout:       3 * time.Minute,
			Workers: map[string]int{
				"cluster":    8,
				"horizontal": 7,
				"some-other": 3,
			},
		},
		Telemetry: &Telemetry{
			StatsiteAddr:                       "some-address",
			StatsdAddr:                         "some-other-address",
			DogStatsDAddr:                      "some-other-other-address",
			DogStatsDTags:                      []string{"most-important-metric"},
			PrometheusMetrics:                  true,
			PrometheusRetentionTime:            48 * time.Hour,
			EnableHostnameLabel:                true,
			DisableHostname:                    true,
			CollectionInterval:                 3 * time.Second,
			CirconusAPIToken:                   "super-secret",
			CirconusAPIApp:                     "secret-app",
			CirconusAPIURL:                     "some-url",
			CirconusSubmissionInterval:         "30s",
			CirconusCheckSubmissionURL:         "some-other-url",
			CirconusCheckID:                    "who-knows",
			CirconusCheckForceMetricActivation: "true",
			CirconusCheckInstanceID:            "some-id",
			CirconusCheckSearchTag:             "some-tag",
			CirconusCheckTags:                  "some-tags",
			CirconusCheckDisplayName:           "some-name",
			CirconusBrokerID:                   "some-id",
			CirconusBrokerSelectTag:            "some-other-tag",
		},
		APMs: []*Plugin{
			{
				Name:   "nomad-apm",
				Driver: "nomad-apm",
			},
			{
				Name:   "prometheus",
				Driver: "prometheus",
				Config: map[string]string{"address": "http://prometheus-new.systems:9090"},
				Args:   []string{"all-the-encryption"},
			},
			{
				Name:   "influx-db",
				Driver: "influx-db",
			},
		},
		Targets: []*Plugin{
			{
				Name:   "nomad-target",
				Driver: "nomad-target",
			},
		},
		Strategies: []*Plugin{
			{
				Name:   "fixed-value",
				Driver: "fixed-value",
			},
			{
				Name:   "pass-through",
				Driver: "pass-through",
			},
			{
				Name:   "target-value",
				Driver: "target-value",
			},
			{
				Name:   "threshold",
				Driver: "threshold",
			},
			{
				Name:   "pid",
				Driver: "pid",
			},
		},
	}

	actualResult := baseCfg.Merge(cfg1)
	actualResult = actualResult.Merge(cfg2)

	// Sort Policy sources to prevent flakiness.
	sort.Slice(actualResult.Policy.Sources, func(i, j int) bool { return actualResult.Policy.Sources[i].Name < actualResult.Policy.Sources[j].Name })

	assert.Equal(t, expectedResult.DynamicApplicationSizing, actualResult.DynamicApplicationSizing)
	assert.Equal(t, expectedResult.HTTP, actualResult.HTTP)
	assert.Equal(t, expectedResult.LogJson, actualResult.LogJson)
	assert.Equal(t, expectedResult.LogLevel, actualResult.LogLevel)
	assert.Equal(t, expectedResult.Nomad, actualResult.Nomad)
	assert.Equal(t, expectedResult.Consul, actualResult.Consul)
	assert.Equal(t, expectedResult.PluginDir, actualResult.PluginDir)
	assert.Equal(t, expectedResult.Policy, actualResult.Policy)
	assert.Equal(t, expectedResult.PolicyEval, actualResult.PolicyEval)
	assert.ElementsMatch(t, expectedResult.APMs, actualResult.APMs)
	assert.ElementsMatch(t, expectedResult.Targets, actualResult.Targets)
	assert.ElementsMatch(t, expectedResult.Strategies, actualResult.Strategies)

	// Test merge on nil config.
	var nilCfg *Agent
	actualResult = nilCfg.Merge(baseCfg)
	assert.Equal(t, baseCfg, actualResult)

	// Test merge on empty config.
	emptyCfg := &Agent{}
	actualResult = emptyCfg.Merge(baseCfg)
	assert.Equal(t, baseCfg, actualResult)
}

func TestAgent_parseFile(t *testing.T) {
	// Should receive a non-nil response as the file doesn't exist.
	assert.NotNil(t, parseFile("/honeybadger/", &Agent{}))

	// Create a temporary file for use.
	fh, err := os.CreateTemp("", "nomad-autoscaler*.hcl")
	assert.Nil(t, err)
	defer os.RemoveAll(fh.Name())

	// Write some nonsense content and expect to receive a non-nil response.
	if _, err := fh.WriteString("¿que?"); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.NotNil(t, parseFile(fh.Name(), &Agent{}))

	// Reset the test file.
	if err := fh.Truncate(0); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := fh.Seek(0, 0); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Write some valid content, and ensure this is read and parsed.
	cfg := &Agent{}

	if _, err := fh.WriteString("plugin_dir = \"/opt/nomad-autoscaler/plugins\""); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Nil(t, parseFile(fh.Name(), cfg))
	assert.Equal(t, "/opt/nomad-autoscaler/plugins", cfg.PluginDir)
}

func TestConfig_Load(t *testing.T) {
	// Fails if the target doesn't exist
	_, err := Load("/honeybadger/")
	assert.NotNil(t, err)

	fh, err := os.CreateTemp("", "nomad-autoscaler*.hcl")
	assert.Nil(t, err)
	defer os.Remove(fh.Name())

	_, err = fh.WriteString("log_level = \"trace\"")
	assert.Nil(t, err)

	// Works on a config file
	cfg, err := Load(fh.Name())
	assert.Nil(t, err)
	assert.Equal(t, "trace", cfg.LogLevel)

	dir, err := os.MkdirTemp("", "nomad-autoscaler")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "config1.hcl")
	assert.Nil(t, os.WriteFile(file1, []byte("plugin_dir = \"/opt/nomad-autoscaler/plugins\""), 0600))

	// Works on config dir
	cfg, err = Load(dir)
	assert.Nil(t, err)
	assert.Equal(t, "/opt/nomad-autoscaler/plugins", cfg.PluginDir)
}

func TestAgent_loadDir(t *testing.T) {
	// Should receive a non-nil response as the dir doesn't exist.
	_, err := loadDir("/honeybadger/")
	assert.NotNil(t, err)

	dir, err := os.MkdirTemp("", "nomad-autoscaler")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	// Returns empty config on empty dir.
	config, err := loadDir(dir)
	assert.Nil(t, err)
	assert.Equal(t, config, &Agent{})

	file1 := filepath.Join(dir, "config1.hcl")
	assert.Nil(t, os.WriteFile(file1, []byte("log_level = \"trace\""), 0600))

	file2 := filepath.Join(dir, "config2.hcl")
	assert.Nil(t, os.WriteFile(file2, []byte("plugin_dir = \"/opt/nomad-autoscaler/plugins\""), 0600))

	file3 := filepath.Join(dir, "config3.hcl")
	assert.Nil(t, os.WriteFile(file3, []byte("¿que?"), 0600))

	// Fails if we have a bad config file.
	_, err = loadDir(dir)
	assert.NotNil(t, err)

	// Remove the invalid config file.
	assert.Nil(t, os.Remove(file3))

	// We should now be able to load as all the configs are valid.
	cfg, err := loadDir(dir)
	assert.Nil(t, err)
	assert.Equal(t, "trace", cfg.LogLevel)
	assert.Equal(t, "/opt/nomad-autoscaler/plugins", cfg.PluginDir)
}

func TestAgent_policySources(t *testing.T) {
	defaultConfig, err := Default()
	require.NoError(t, err)

	// Create a temporary file for use.
	fh, err := os.CreateTemp("", "nomad-autoscaler*.hcl")
	require.NoError(t, err)
	defer os.RemoveAll(fh.Name())

	// Enabled by default if block is present.
	cfg := &Agent{}
	nomadSourceCfg := `
policy {
  source "nomad" {}
}`

	_, err = fh.WriteString(nomadSourceCfg)
	require.NoError(t, err)
	require.NoError(t, parseFile(fh.Name(), cfg))

	expected := []*PolicySource{
		{
			Name:    "nomad",
			Enabled: ptr.BoolToPtr(true),
		},
	}
	assert.ElementsMatch(t, expected, cfg.Policy.Sources)

	// Disable "nomad" policy source.
	cfg = &Agent{}

	noNomadSourceCfg := `
policy {
  source "nomad" {
    enabled = false
  }
}`

	// Reset the test file.
	err = fh.Truncate(0)
	require.NoError(t, err)
	_, err = fh.Seek(0, 0)
	require.NoError(t, err)

	_, err = fh.WriteString(noNomadSourceCfg)
	require.NoError(t, err)
	require.NoError(t, parseFile(fh.Name(), cfg))

	result := defaultConfig.Merge(cfg)
	expected = []*PolicySource{
		{
			Name:    "file",
			Enabled: ptr.BoolToPtr(true),
		},
		{
			Name:    "nomad",
			Enabled: ptr.BoolToPtr(false),
		},
	}
	assert.ElementsMatch(t, expected, result.Policy.Sources)

	// Disable "file" policy source.
	cfg = &Agent{}

	noFileSourceCfg := `
policy {
  source "file" {
    enabled = false
  }
}`

	// Reset the test file.
	err = fh.Truncate(0)
	require.NoError(t, err)
	_, err = fh.Seek(0, 0)
	require.NoError(t, err)

	_, err = fh.WriteString(noFileSourceCfg)
	require.NoError(t, err)
	require.NoError(t, parseFile(fh.Name(), cfg))

	result = defaultConfig.Merge(cfg)
	expected = []*PolicySource{
		{
			Name:    "file",
			Enabled: ptr.BoolToPtr(false),
		},
		{
			Name:    "nomad",
			Enabled: ptr.BoolToPtr(true),
		},
	}
	assert.ElementsMatch(t, expected, result.Policy.Sources)
}
