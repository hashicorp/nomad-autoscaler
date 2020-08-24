package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Default(t *testing.T) {
	def, err := Default()
	assert.Nil(t, err)
	assert.NotNil(t, def)
	assert.False(t, def.LogJson)
	assert.Equal(t, def.LogLevel, "info")
	assert.True(t, strings.HasSuffix(def.PluginDir, "/plugins"))
	assert.Equal(t, def.Policy.DefaultEvaluationInterval, 10*time.Second)
	assert.Equal(t, def.Nomad.Address, "http://127.0.0.1:4646")
	assert.Equal(t, "127.0.0.1", def.HTTP.BindAddress)
	assert.Equal(t, 8080, def.HTTP.BindPort)
	assert.Equal(t, def.Policy.DefaultCooldown, 5*time.Minute)
	assert.Len(t, def.APMs, 1)
	assert.Len(t, def.Targets, 1)
	assert.Len(t, def.Strategies, 1)
	assert.Equal(t, 1*time.Second, def.Telemetry.CollectionInterval)
}

func TestAgent_Merge(t *testing.T) {
	baseCfg, err := Default()
	assert.Nil(t, err)

	cfg1 := &Agent{
		PluginDir: "/opt/nomad-autoscaler/plugins",
		HTTP: &HTTP{
			BindAddress: "scaler.nomad",
		},
		Nomad: &Nomad{
			Address: "http://nomad.systems:4646",
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
		LogLevel:  "trace",
		LogJson:   true,
		PluginDir: "/var/lib/nomad-autoscaler/plugins",
		HTTP: &HTTP{
			BindPort: 4646,
		},
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
		LogLevel:  "trace",
		LogJson:   true,
		PluginDir: "/var/lib/nomad-autoscaler/plugins",
		HTTP: &HTTP{
			BindAddress: "scaler.nomad",
			BindPort:    4646,
		},
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
				Name:   "target-value",
				Driver: "target-value",
			},
			{
				Name:   "pid",
				Driver: "pid",
			},
		},
	}

	actualResult := baseCfg.Merge(cfg1)
	actualResult = actualResult.Merge(cfg2)

	assert.Equal(t, expectedResult.HTTP, actualResult.HTTP)
	assert.Equal(t, expectedResult.LogJson, actualResult.LogJson)
	assert.Equal(t, expectedResult.LogLevel, actualResult.LogLevel)
	assert.Equal(t, expectedResult.Nomad, actualResult.Nomad)
	assert.Equal(t, expectedResult.PluginDir, actualResult.PluginDir)
	assert.Equal(t, expectedResult.Policy, actualResult.Policy)
	assert.ElementsMatch(t, expectedResult.APMs, actualResult.APMs)
	assert.ElementsMatch(t, expectedResult.Targets, actualResult.Targets)
	assert.ElementsMatch(t, expectedResult.Strategies, actualResult.Strategies)
}

func TestAgent_parseFile(t *testing.T) {
	// Should receive a non-nil response as the file doesn't exist.
	assert.NotNil(t, parseFile("/honeybadger/", &Agent{}))

	// Create a temporary file for use.
	fh, err := ioutil.TempFile("", "nomad-autoscaler*.hcl")
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

	fh, err := ioutil.TempFile("", "nomad-autoscaler*.hcl")
	assert.Nil(t, err)
	defer os.Remove(fh.Name())

	_, err = fh.WriteString("log_level = \"trace\"")
	assert.Nil(t, err)

	// Works on a config file
	cfg, err := Load(fh.Name())
	assert.Nil(t, err)
	assert.Equal(t, "trace", cfg.LogLevel)

	dir, err := ioutil.TempDir("", "nomad-autoscaler")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "config1.hcl")
	assert.Nil(t, ioutil.WriteFile(file1, []byte("plugin_dir = \"/opt/nomad-autoscaler/plugins\""), 0600))

	// Works on config dir
	cfg, err = Load(dir)
	assert.Nil(t, err)
	assert.Equal(t, "/opt/nomad-autoscaler/plugins", cfg.PluginDir)
}

func TestAgent_loadDir(t *testing.T) {
	// Should receive a non-nil response as the dir doesn't exist.
	_, err := loadDir("/honeybadger/")
	assert.NotNil(t, err)

	dir, err := ioutil.TempDir("", "nomad-autoscaler")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	// Returns empty config on empty dir.
	config, err := loadDir(dir)
	assert.Nil(t, err)
	assert.Equal(t, config, &Agent{})

	file1 := filepath.Join(dir, "config1.hcl")
	assert.Nil(t, ioutil.WriteFile(file1, []byte("log_level = \"trace\""), 0600))

	file2 := filepath.Join(dir, "config2.hcl")
	assert.Nil(t, ioutil.WriteFile(file2, []byte("plugin_dir = \"/opt/nomad-autoscaler/plugins\""), 0600))

	file3 := filepath.Join(dir, "config3.hcl")
	assert.Nil(t, ioutil.WriteFile(file3, []byte("¿que?"), 0600))

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
