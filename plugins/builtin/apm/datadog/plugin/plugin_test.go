package plugin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPMPlugin_SetConfig(t *testing.T) {
	testCases := []struct {
		inputConfig          map[string]string
		keyEnvVar            string
		appEnvVar            string
		expectOutput         error
		expectedContextKey   interface{}
		expectedContextValue interface{}
		name                 string
	}{
		{
			inputConfig:          map[string]string{},
			keyEnvVar:            "",
			appEnvVar:            "",
			expectOutput:         errors.New(`"dd_api_key" config value cannot be empty`),
			expectedContextValue: nil,
			name:                 "no required config parameters set",
		},
		{
			inputConfig:          map[string]string{"dd_api_key": "fake-api-key"},
			keyEnvVar:            "",
			appEnvVar:            "",
			expectOutput:         errors.New(`"dd_app_key" config value cannot be empty`),
			expectedContextValue: nil,
			name:                 "partial require parameters set by config map",
		},
		{
			inputConfig:          map[string]string{},
			keyEnvVar:            "env-var-fake-api-key",
			appEnvVar:            "",
			expectOutput:         errors.New(`"dd_app_key" config value cannot be empty`),
			expectedContextValue: nil,
			name:                 "partial require parameters set by env var",
		},

		{
			inputConfig:        map[string]string{"dd_api_key": "fake-api-key", "dd_app_key": "some-app"},
			keyEnvVar:          "",
			appEnvVar:          "",
			expectOutput:       nil,
			expectedContextKey: datadog.ContextAPIKeys,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "all required config parameters set by config map",
		},
		{
			inputConfig:        map[string]string{"dd_api_key": "fake-api-key", "dd_app_key": "some-app"},
			keyEnvVar:          "env-var-fake-api-key",
			appEnvVar:          "env-var-some-app",
			expectOutput:       nil,
			expectedContextKey: datadog.ContextAPIKeys,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "all required config parameters set by both config map and env vars",
		},
		{
			inputConfig:        map[string]string{},
			keyEnvVar:          "env-var-fake-api-key",
			appEnvVar:          "env-var-some-app",
			expectOutput:       nil,
			expectedContextKey: datadog.ContextAPIKeys,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "env-var-fake-api-key"},
				"appKeyAuth": {Key: "env-var-some-app"},
			},
			name: "all required config parameters set by env vars",
		},
		{
			inputConfig:        map[string]string{"dd_api_key": "fake-api-key"},
			keyEnvVar:          "",
			appEnvVar:          "env-var-some-app",
			expectOutput:       nil,
			expectedContextKey: datadog.ContextAPIKeys,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "env-var-some-app"},
			},
			name: "key set by config map, app set by env var",
		},
		{
			inputConfig:        map[string]string{"dd_app_key": "some-app"},
			keyEnvVar:          "env-var-fake-api-key",
			appEnvVar:          "",
			expectOutput:       nil,
			expectedContextKey: datadog.ContextAPIKeys,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "env-var-fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "app set by config map, key set by env var",
		},
		{
			inputConfig:        map[string]string{"site": "app.datadoghq.eu"},
			expectOutput:       nil,
			expectedContextKey: datadog.ContextServerVariables,
			expectedContextValue: map[string]string{
				"site": "app.datadoghq.eu",
			},
			name: "site set by config map",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apmPlugin := APMPlugin{logger: hclog.NewNullLogger()}

			// Set the environment variables if we are testing these.
			if tc.keyEnvVar != "" {
				assert.Nil(t, os.Setenv("DD_API_KEY", tc.keyEnvVar), tc.name)
			}
			if tc.appEnvVar != "" {
				assert.Nil(t, os.Setenv("DD_APP_KEY", tc.appEnvVar), tc.name)
			}

			// Perform the function call.
			actualOutput := apmPlugin.SetConfig(tc.inputConfig)
			assert.Equal(t, tc.expectOutput, actualOutput, tc.name)

			// Check the stored context and the client. If we expect to have a
			// non-nil context then we should have a non-nil client and vice
			// versa.
			if tc.expectedContextValue != nil {
				assert.Equal(t, tc.expectedContextValue, apmPlugin.clientCtx.Value(tc.expectedContextKey), tc.name)
				assert.NotNil(t, apmPlugin.client, tc.name)
			} else {
				assert.Nil(t, apmPlugin.clientCtx, tc.name)
				assert.Nil(t, apmPlugin.client, tc.name)
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
				configKeyClientAPPKey: "app",
				configKeyClientAPIKey: "key",
			},
			query: "avg:nomad.client.allocated.memory",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				// Check auth headers.
				require.Equal(t, "app", r.Header.Get("DD-APPLICATION-KEY"))
				require.Equal(t, "key", r.Header.Get("DD-API-KEY"))

				// Check query params.
				qp := r.URL.Query()
				require.Equal(t, "avg:nomad.client.allocated.memory", qp.Get("query"))
				require.Equal(t, "1600000000", qp.Get("from"))
				require.Equal(t, "1610000000", qp.Get("to"))
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 63)
			},
		},
		{
			name:    "handle null values",
			fixture: "query_null_result.json",
			pluginConfig: map[string]string{
				configKeyClientAPPKey: "app",
				configKeyClientAPIKey: "key",
			},
			query: "avg:nomad.client.allocated.memory",
			timeRange: sdk.TimeRange{
				From: time.Unix(1660000000, 0),
				To:   time.Unix(1670000000, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 20)
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

			srvURL, err := url.Parse(srv.URL)
			require.NoError(t, err)

			plugin := NewDatadogPlugin(hclog.NewNullLogger())

			// Configure plugin to talk to the test server.
			plugin.(*APMPlugin).ddConfigCallback = func(config *datadog.Configuration) {
				config.Host = srvURL.Host
				config.Scheme = srvURL.Scheme
			}

			err = plugin.SetConfig(tc.pluginConfig)
			require.NoError(t, err)

			metrics, err := plugin.Query(tc.query, tc.timeRange)
			if tc.validateMetrics != nil {
				tc.validateMetrics(t, metrics, err)
			}
		})
	}
}
