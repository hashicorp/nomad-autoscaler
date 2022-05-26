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
		inputConfig  map[string]string
		expectOutput error
		name         string
	}{
		{
			inputConfig:  map[string]string{},
			expectOutput: errors.New(`"address" config value cannot be empty`),
			name:         "no required config parameters set",
		},
		{
			inputConfig:  map[string]string{"address": "\n\n"},
			expectOutput: errors.New(`failed to initialize Prometheus client: parse "\n\n": net/url: invalid control character in URL`),
			name:         "required config parameters set but value malformed",
		},
		{
			inputConfig:  map[string]string{"address": "http://127.0.0.1:9090"},
			expectOutput: nil,
			name:         "required and valid config parameters set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apmPlugin := APMPlugin{logger: hclog.NewNullLogger()}

			actualOutput := apmPlugin.SetConfig(tc.inputConfig)
			assert.Equal(t, tc.expectOutput, actualOutput, tc.name)

			// If the function call did not return an error, we should have a
			// non-nil Prometheus client.
			if actualOutput == nil {
				assert.NotNil(t, apmPlugin.client)
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
			fixture: "query_range_200.json",
			pluginConfig: map[string]string{
				configKeyBasicAuthUser:     "user",
				configKeyBasicAuthPassword: "pass",
				"header_test":              "true",
				configKeyCACert:            "./test-fixtures/ca.crt",
				configKeySkipVerify:        "true",
			},
			query: "nomad_client_allocated_memory",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				// Vefify auth.
				username, password, ok := r.BasicAuth()
				require.True(t, ok)
				require.Equal(t, "user", username)
				require.Equal(t, "pass", password)

				// Verify request body.
				r.ParseForm()
				require.Equal(t, "nomad_client_allocated_memory", r.FormValue("query"))
				require.Equal(t, "1600000000", r.FormValue("start"))
				require.Equal(t, "1610000000", r.FormValue("end"))

				// Verify custom headers.
				require.Equal(t, "true", r.Header.Get("test"))
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				require.NoError(t, err)
				require.Len(t, m, 31)
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

			// Set plugin to talk to the test server.
			tc.pluginConfig[configKeyAddress] = srv.URL

			plugin := NewPrometheusPlugin(hclog.NewNullLogger())
			err := plugin.SetConfig(tc.pluginConfig)
			require.NoError(t, err)

			metrics, err := plugin.Query(tc.query, tc.timeRange)
			if tc.validateMetrics != nil {
				tc.validateMetrics(t, metrics, err)
			}
		})
	}
}
