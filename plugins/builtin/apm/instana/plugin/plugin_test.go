// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

var (
	testFrom  = time.Date(2024, 5, 29, 12, 0, 0, 0, time.UTC)
	testTo    = testFrom.Add(5 * time.Minute)
	testQuery = `{"plugin":"host","metrics":["cpu.used"]}`
)

// TestAPMPlugin_SetConfig verifies that SetConfig accepts valid configurations
// and rejects invalid ones, covering required fields, URL validation, and the
// api_token environment variable fallback.
func TestAPMPlugin_SetConfig(t *testing.T) {
	testCases := []struct {
		name        string
		inputConfig map[string]string
		envToken    string
		expectErr   string
	}{
		{
			name:        "empty config - missing endpoint",
			inputConfig: map[string]string{},
			expectErr:   "endpoint config value cannot be empty",
		},
		{
			name: "missing api_token and no env var",
			inputConfig: map[string]string{
				configKeyEndpoint: "https://test-acme.instana.io",
			},
			expectErr: "api_token config value cannot be empty",
		},
		{
			name: "api_token supplied via env var",
			inputConfig: map[string]string{
				configKeyEndpoint: "https://test-acme.instana.io",
			},
			envToken:  "env-token-123",
			expectErr: "",
		},
		{
			name: "invalid endpoint - no scheme",
			inputConfig: map[string]string{
				configKeyEndpoint: "test-acme.instana.io",
				configKeyAPIToken: "my-token",
			},
			expectErr: "endpoint must be a valid absolute URL",
		},
		{
			name: "invalid endpoint - relative path",
			inputConfig: map[string]string{
				configKeyEndpoint: "/just/a/path",
				configKeyAPIToken: "my-token",
			},
			expectErr: "endpoint must be a valid absolute URL",
		},
		{
			name: "valid config - both keys set",
			inputConfig: map[string]string{
				configKeyEndpoint: "https://test-acme.instana.io",
				configKeyAPIToken: "my-token",
			},
			expectErr: "",
		},
		{
			name: "valid config - self-hosted endpoint",
			inputConfig: map[string]string{
				configKeyEndpoint: "https://instana.mycompany.com",
				configKeyAPIToken: "my-token",
			},
			expectErr: "",
		},
		{
			name: "config key takes precedence over env var",
			inputConfig: map[string]string{
				configKeyEndpoint: "https://test-acme.instana.io",
				configKeyAPIToken: "config-token",
			},
			envToken:  "env-token",
			expectErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envToken != "" {
				// t.Setenv sets the value and automatically restores the
				// original state once the subtest finishes.
				t.Setenv(envKeyAPIToken, tc.envToken)
			}

			p := &APMPlugin{
				logger: hclog.NewNullLogger(),
				client: newInstanaClient(),
			}
			err := p.SetConfig(tc.inputConfig)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
				// state must not be partially committed on error
				must.Nil(t, p.client.baseURL)
				must.Eq(t, "", p.client.apiToken)
			} else {
				must.NoError(t, err)
				must.NotNil(t, p.client.baseURL)
				must.NotEq(t, "", p.client.apiToken)
			}
		})
	}
}

// Test_parseItems verifies the pure conversion logic: one sdk.TimestampedMetrics
// slice is produced per (entity snapshot × metric ID) pair, and no data point
// is silently dropped regardless of its value.
func Test_parseItems(t *testing.T) {
	t.Run("empty items returns nil", func(t *testing.T) {
		result := parseItems(nil)
		must.Nil(t, result)
	})

	t.Run("two items one metric key each produces two series", func(t *testing.T) {
		items := []instanaMetricItem{
			{
				SnapshotID: "snap-a",
				Metrics: map[string][][2]float64{
					"cpu.used": {{1717000000000, 10.0}, {1717000060000, 20.0}},
				},
			},
			{
				SnapshotID: "snap-b",
				Metrics: map[string][][2]float64{
					"cpu.used": {{1717000000000, 30.0}, {1717000060000, 40.0}},
				},
			},
		}
		result := parseItems(items)
		must.Eq(t, 2, len(result))
		must.Eq(t, 2, len(result[0]))
		must.Eq(t, 2, len(result[1]))
	})

	t.Run("single item with two metric keys produces two series", func(t *testing.T) {
		items := []instanaMetricItem{
			{
				SnapshotID: "snap-a",
				Metrics: map[string][][2]float64{
					"cpu.used":    {{1717000000000, 10.0}},
					"memory.used": {{1717000000000, 512.0}},
				},
			},
		}
		result := parseItems(items)
		// Map iteration order is non-deterministic; only the count is asserted.
		must.Eq(t, 2, len(result))
	})

	t.Run("zero-value metric point is included not filtered", func(t *testing.T) {
		items := []instanaMetricItem{
			{
				SnapshotID: "snap-a",
				Metrics: map[string][][2]float64{
					"cpu.used": {
						{1717000000000, 42.5},
						{1717000060000, 0},
						{1717000120000, 43.7},
					},
				},
			},
		}
		result := parseItems(items)
		must.Eq(t, 1, len(result))
		must.Eq(t, 3, len(result[0]))
		must.Eq(t, sdk.TimestampedMetric{
			Timestamp: time.UnixMilli(1717000060000),
			Value:     0,
		}, result[0][1])
	})
}
