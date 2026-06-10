// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/test/must"
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
				t.Setenv(envKeyAPIToken, tc.envToken)
			}

			p := &APMPlugin{logger: hclog.NewNullLogger()}
			err := p.SetConfig(tc.inputConfig)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
				must.Nil(t, p.cfg.BaseURL)
				must.Eq(t, "", p.cfg.APIToken)
			} else {
				must.NoError(t, err)
				must.NotNil(t, p.cfg.BaseURL)
				must.NotEq(t, "", p.cfg.APIToken)
				must.NotNil(t, p.client)
			}
		})
	}
}
