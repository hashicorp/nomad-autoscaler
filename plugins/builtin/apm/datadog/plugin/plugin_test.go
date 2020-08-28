package plugin

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestAPMPlugin_SetConfig(t *testing.T) {
	testCases := []struct {
		inputConfig          map[string]string
		keyEnvVar            string
		appEnvVar            string
		expectOutput         error
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
			inputConfig:  map[string]string{"dd_api_key": "fake-api-key", "dd_app_key": "some-app"},
			keyEnvVar:    "",
			appEnvVar:    "",
			expectOutput: nil,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "all required config parameters set by config map",
		},
		{
			inputConfig:  map[string]string{"dd_api_key": "fake-api-key", "dd_app_key": "some-app"},
			keyEnvVar:    "env-var-fake-api-key",
			appEnvVar:    "env-var-some-app",
			expectOutput: nil,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "all required config parameters set by both config map and env vars",
		},
		{
			inputConfig:  map[string]string{},
			keyEnvVar:    "env-var-fake-api-key",
			appEnvVar:    "env-var-some-app",
			expectOutput: nil,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "env-var-fake-api-key"},
				"appKeyAuth": {Key: "env-var-some-app"},
			},
			name: "all required config parameters set by env vars",
		},
		{
			inputConfig:  map[string]string{"dd_api_key": "fake-api-key"},
			keyEnvVar:    "",
			appEnvVar:    "env-var-some-app",
			expectOutput: nil,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "fake-api-key"},
				"appKeyAuth": {Key: "env-var-some-app"},
			},
			name: "key set by config map, app set by env var",
		},
		{
			inputConfig:  map[string]string{"dd_app_key": "some-app"},
			keyEnvVar:    "env-var-fake-api-key",
			appEnvVar:    "",
			expectOutput: nil,
			expectedContextValue: map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "env-var-fake-api-key"},
				"appKeyAuth": {Key: "some-app"},
			},
			name: "app set by config map, key set by env var",
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
				assert.Equal(t, tc.expectedContextValue, apmPlugin.clientCtx.Value(datadog.ContextAPIKeys), tc.name)
				assert.NotNil(t, apmPlugin.client, tc.name)
			} else {
				assert.Nil(t, apmPlugin.clientCtx, tc.name)
				assert.Nil(t, apmPlugin.client, tc.name)
			}
		})
	}
}

func Test_parseDatadogRawQuery(t *testing.T) {
	testCases := []struct {
		name               string
		rawQuery           string
		expectedQuery      string
		expectedTimeWindow time.Duration
		expectError        bool
	}{
		{
			name:               "simple query",
			rawQuery:           "FROM=1m;TO=0m;QUERY=foo",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        false,
		},
		{
			name:               "change order of components",
			rawQuery:           "TO=0m;QUERY=foo;FROM=1m",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        false,
		},
		{
			name:               "error: trailing delimiter",
			rawQuery:           "FROM=1m;TO=0m;QUERY=foo;;",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: from after to",
			rawQuery:           "FROM=1m;TO=3m;QUERY=foo",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: empty query",
			rawQuery:           "FROM=1m;TO=3m;QUERY=",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing query",
			rawQuery:           "FROM=1m;TO=3m",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing from",
			rawQuery:           "TO=3m;QUERY=foo",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing from and to",
			rawQuery:           "QUERY=foo",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := parseRawQuery(tc.rawQuery)
			if tc.expectError {
				assert.NotNil(t, err, tc.rawQuery, tc.name)
				return
			}
			assert.Nil(t, err, tc.name)
			assert.Equal(t, tc.expectedQuery, actualOutput.query, tc.name)
			assert.Equal(t, tc.expectedTimeWindow, actualOutput.to.Sub(actualOutput.from), tc.name)
		})
	}
}
