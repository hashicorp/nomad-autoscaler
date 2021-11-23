package plugin

import (
	"net/http"
	"net/http/httptest"
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
	rt := newPluginRoudTripper(cfg)
	client := &http.Client{Transport: rt}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	// Make a request to run the tests.
	_, err := client.Get(server.URL)
	require.NoError(t, err)
}
