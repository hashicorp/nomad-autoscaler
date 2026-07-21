// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/shoenig/test/must"
)

// TestAPMPlugin_SetAuthHeader verifies that setAuthHeader sets the correct
// Authorization header for each auth mode: unauthenticated, Basic (username
// only and username+password), and JWT Bearer via shared_secret.
func TestAPMPlugin_SetAuthHeader(t *testing.T) {
	testCases := []struct {
		name         string
		cfg          pluginConfig
		expectBasic  bool
		expectBearer bool
	}{
		{
			name: "unauthenticated — no credentials set",
			cfg:  pluginConfig{},
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
			must.NoError(t, err)

			authHeader := req.Header.Get("Authorization")

			switch {
			case tc.expectBasic:
				must.NotEq(t, "", authHeader)
				must.True(t, strings.HasPrefix(authHeader, "Basic "), must.Sprintf("expected Basic auth scheme, got: %s", authHeader))

			case tc.expectBearer:
				must.NotEq(t, "", authHeader)
				must.True(t, strings.HasPrefix(authHeader, "Bearer "), must.Sprintf("expected Bearer scheme, got: %s", authHeader))

				rawToken := strings.TrimPrefix(authHeader, "Bearer ")
				tok, err := jwt.Parse(rawToken, func(jwtTok *jwt.Token) (interface{}, error) {
					_, ok := jwtTok.Method.(*jwt.SigningMethodHMAC)
					must.True(t, ok, must.Sprint("expected HS256 signing method"))
					return []byte(tc.cfg.SharedSecret), nil
				})
				must.NoError(t, err)
				must.True(t, tok.Valid)

				claims, ok := tok.Claims.(jwt.MapClaims)
				must.True(t, ok)
				must.Eq(t, tc.cfg.Username, claims["username"].(string))

			default:
				must.Eq(t, "", authHeader, must.Sprint("expected no Authorization header"))
			}
		})
	}
}
