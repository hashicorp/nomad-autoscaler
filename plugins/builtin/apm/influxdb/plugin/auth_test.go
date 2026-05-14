// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPMPlugin_SetAuthHeader(t *testing.T) {
	testCases := []struct {
		name         string
		cfg          pluginConfig
		expectAuth   string // expected Authorization header value; "" means no header
		expectBasic  bool   // true if basic auth is expected (checked via header prefix)
		expectBearer bool   // true if bearer JWT is expected
	}{
		{
			name:       "unauthenticated — no credentials set",
			cfg:        pluginConfig{},
			expectAuth: "",
		},
		{
			name: "basic auth — username and password",
			cfg: pluginConfig{
				Username: "user",
				Password: "pass",
			},
			expectBasic: true,
		},
		{
			name: "basic auth — username only",
			cfg: pluginConfig{
				Username: "user",
			},
			expectBasic: true,
		},
		{
			name: "JWT bearer — shared_secret with username",
			cfg: pluginConfig{
				Username:     "autoscaler",
				SharedSecret: "my-secret",
				TokenTTL:     time.Hour,
			},
			expectBearer: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &APMPlugin{
				logger: hclog.NewNullLogger(),
				cfg:    tc.cfg,
			}

			req := httptest.NewRequest(http.MethodGet, "http://localhost:8086/query", nil)
			err := p.setAuthHeader(req)
			require.NoError(t, err)

			authHeader := req.Header.Get("Authorization")

			switch {
			case tc.expectAuth == "" && !tc.expectBasic && !tc.expectBearer:
				assert.Empty(t, authHeader, "expected no Authorization header")

			case tc.expectBasic:
				require.NotEmpty(t, authHeader)
				assert.True(t, strings.HasPrefix(authHeader, "Basic "), "expected Basic auth scheme, got: %s", authHeader)

			case tc.expectBearer:
				require.NotEmpty(t, authHeader)
				require.True(t, strings.HasPrefix(authHeader, "Bearer "), "expected Bearer scheme, got: %s", authHeader)

				rawToken := strings.TrimPrefix(authHeader, "Bearer ")
				tok, err := jwt.Parse(rawToken, func(jwtTok *jwt.Token) (interface{}, error) {
					_, ok := jwtTok.Method.(*jwt.SigningMethodHMAC)
					require.True(t, ok, "expected HS256 signing method")
					return []byte(tc.cfg.SharedSecret), nil
				})
				require.NoError(t, err)
				require.True(t, tok.Valid)

				claims, ok := tok.Claims.(jwt.MapClaims)
				require.True(t, ok)
				assert.Equal(t, tc.cfg.Username, claims["username"])
			}
		})
	}
}

func TestAPMPlugin_JWT_Caching(t *testing.T) {
	p := &APMPlugin{
		logger: hclog.NewNullLogger(),
		cfg: pluginConfig{
			Username:     "autoscaler",
			SharedSecret: "cache-test-secret",
			TokenTTL:     time.Hour,
		},
	}

	// First call — token is generated.
	tok1, err := p.getOrRefreshJWT()
	require.NoError(t, err)
	require.NotEmpty(t, tok1)

	// Second call within TTL — same token returned (cached).
	tok2, err := p.getOrRefreshJWT()
	require.NoError(t, err)
	require.Equal(t, tok1, tok2, "expected cached token to be reused")

	// Sleep 1s so the next token gets a different exp. JWT timestamps are
	// whole seconds, so same-second regeneration produces an identical string.
	time.Sleep(time.Second)

	// Simulate expiry by rewinding tokenExpiry into the refresh window.
	p.jwtMu.Lock()
	p.tokenExpiry = time.Now().Add(10 * time.Second) // inside 30s refresh window
	p.jwtMu.Unlock()

	// Third call — token must be refreshed with a new exp.
	tok3, err := p.getOrRefreshJWT()
	require.NoError(t, err)
	require.NotEmpty(t, tok3)
	// exp is recomputed on each generation, so a fresh token is always a different string
	require.NotEqual(t, tok1, tok3, "expected a newly generated token after entering refresh window")
	// make sure the new token is actually valid
	parsed, err := jwt.Parse(tok3, func(jwtTok *jwt.Token) (interface{}, error) {
		return []byte("cache-test-secret"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
}
