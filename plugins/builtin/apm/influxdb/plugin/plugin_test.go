// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPMPlugin_SetConfig(t *testing.T) {
	testCases := []struct {
		name         string
		inputConfig  map[string]string
		expectOutput error
	}{
		{
			name:         "no required config parameters set",
			inputConfig:  map[string]string{},
			expectOutput: errors.New(`"address" config value cannot be empty`), // Todo check project specific syntax for error message matching
		},
		{
			name:         "missing database",
			inputConfig:  map[string]string{configKeyAddress: "http://localhost:8086"},
			expectOutput: errors.New(`"database" config value cannot be empty`),
		},
		{
			name: "unsupported version 2",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "2",
			},
			expectOutput: errors.New(`influxdb version "2" is not yet supported: only version "1" is currently implemented`),
		},
		{
			name: "unsupported version 3",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "3",
			},
			expectOutput: errors.New(`influxdb version "3" is not yet supported: only version "1" is currently implemented`),
		},
		{
			name: "invalid version",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "invalid",
			},
			expectOutput: errors.New(`invalid influxdb version "invalid": only version "1" is supported`),
		},
		{
			name: "all required config parameters set by database",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
			},
			expectOutput: nil,
		},
		{
			name: "all required config parameters set by db",
			inputConfig: map[string]string{
				configKeyAddress: "http://localhost:8086",
				configKeyDB:      "telegraf",
			},
			expectOutput: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apmPlugin := APMPlugin{logger: hclog.NewNullLogger()}

			actualOutput := apmPlugin.SetConfig(tc.inputConfig)
			assert.Equal(t, tc.expectOutput, actualOutput)

			if tc.expectOutput == nil {
				assert.NotNil(t, apmPlugin.client)
				assert.NotNil(t, apmPlugin.baseURL)
			} else {
				assert.Nil(t, apmPlugin.client)
				assert.Nil(t, apmPlugin.baseURL)
			}
		})
	}
}

func TestAPMPlugin_Query(t *testing.T) {
	testCases := []struct {
		name            string
		fixture         string
		pluginConfig    map[string]string
		query           string
		timeRange       sdk.TimeRange
		validateRequest func(*testing.T, *http.Request)
		validateMetrics func(*testing.T, sdk.TimestampedMetrics, error)
	}{
		{
			name:    "success",
			fixture: "query_200.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
				configKeyUsername: "user",
				configKeyPassword: "pass",
			},
			query: "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 10m",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				require.Equal(t, "/query", r.URL.Path)
				qp := r.URL.Query()
				require.Equal(t, "telegraf", qp.Get("db"))
				require.Equal(t, "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 10m", qp.Get("q"))
				require.Equal(t, "s", qp.Get("epoch"))
				require.Equal(t, "user", qp.Get("u"))
				require.Equal(t, "pass", qp.Get("p"))
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 3)
			},
		},
		{
			name:    "handle null values",
			fixture: "query_null_result.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT mean(usage_idle) FROM cpu",
			timeRange: sdk.TimeRange{
				From: time.Unix(1660000000, 0),
				To:   time.Unix(1670000000, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 2)
			},
		},
		{
			name:    "multiple streams returns error",
			fixture: "query_multiple_streams.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT usage_idle FROM cpu",
			timeRange: sdk.TimeRange{
				From: time.Unix(1660000000, 0),
				To:   time.Unix(1670000000, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "only 1 is expected")
			},
		},
		{
			name:    "query memory with mean aggregation",
			fixture: "query_memory.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT mean(used_percent) FROM mem WHERE time > now() - 5m",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1600000200, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 4)
				// Verify values are parsed correctly from "mean" column
				require.Equal(t, 75.5, m[0].Value)
				require.Equal(t, 78.2, m[1].Value)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.validateRequest != nil {
					tc.validateRequest(t, r)
				}
				http.ServeFile(w, r, path.Join("./test-fixtures", tc.fixture))
			}))
			defer srv.Close()

			plugin := NewInfluxDBPlugin(hclog.NewNullLogger())
			err := plugin.SetConfig(map[string]string{
				configKeyAddress:  srv.URL,
				configKeyDatabase: tc.pluginConfig[configKeyDatabase],
				configKeyUsername: tc.pluginConfig[configKeyUsername],
				configKeyPassword: tc.pluginConfig[configKeyPassword],
			})
			require.NoError(t, err)

			metrics, err := plugin.Query(tc.query, tc.timeRange)
			if tc.validateMetrics != nil {
				tc.validateMetrics(t, metrics, err)
			}
		})
	}
}
