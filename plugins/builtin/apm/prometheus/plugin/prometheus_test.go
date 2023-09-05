// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPMPlugin_roundTripper(t *testing.T) {
	cfg := map[string]string{
		"basic_auth_user":        "user",
		"basic_auth_password":    "secret",
		"header_X-Scope-OrgID":   "my-org",
		"header_X-Custom-Header": "header value",
	}

	// Run tests inside the handler where we can check the values actually
	// passed in the request.
	handler := func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		assert.Equal(t, "user", username)
		assert.Equal(t, "secret", password)

		assert.Equal(t, r.Header.Get("X-Scope-OrgID"), "my-org")
		assert.Equal(t, r.Header.Get("X-Custom-Header"), "header value")
	}

	// Setup round tripper and an HTTP client to use for testing.
	rt := newPluginRoudTripper(cfg, nil)
	client := &http.Client{Transport: rt}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	// Make a request to run the tests.
	_, err := client.Get(server.URL)
	require.NoError(t, err)
}

func TestAPMPlugin_roundTripperTLS(t *testing.T) {
	// Setup test HTTPS server.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	// Write CA cert into a temporary file.
	caCertFile, err := os.CreateTemp("", "ca_cert*.pem")
	require.NoError(t, err)
	defer os.RemoveAll(caCertFile.Name())

	err = pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, err)

	// Write invalid CA cert into a temporary file.
	invalidcaCertFile, err := os.CreateTemp("", "ca_cert*.pem")
	require.NoError(t, err)
	defer os.RemoveAll(invalidcaCertFile.Name())

	invalidcaCertFile.Write([]byte("not a cert"))

	testCases := []struct {
		name                  string
		cfg                   map[string]string
		expectConnError       bool
		expectValidationError bool
	}{
		{
			name:                  "no tls",
			cfg:                   map[string]string{},
			expectConnError:       true,
			expectValidationError: false,
		},
		{
			name: "skip verify",
			cfg: map[string]string{
				"skip_verify": "true",
			},
			expectConnError:       false,
			expectValidationError: false,
		},
		{
			name: "set CA cert",
			cfg: map[string]string{
				"ca_cert": caCertFile.Name(),
			},
			expectConnError:       false,
			expectValidationError: false,
		},
		{
			name: "invalid skip verify",
			cfg: map[string]string{
				"skip_verify": "not-valid",
			},
			expectValidationError: true,
		},
		{
			name: "invalid CA path",
			cfg: map[string]string{
				"ca_cert": "not-valid",
			},
			expectValidationError: true,
		},
		{
			name: "invalid CA cert",
			cfg: map[string]string{
				"ca_cert": invalidcaCertFile.Name(),
			},
			expectValidationError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup round tripper and an HTTP client to use for testing.
			tlsConfig, err := generateTLSConfig(tc.cfg)
			if tc.expectValidationError {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			rt := newPluginRoudTripper(tc.cfg, tlsConfig)
			client := &http.Client{Transport: rt}

			// Make a request to run the tests.
			_, err = client.Get(server.URL)
			if tc.expectConnError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
