// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// configKeySharedSecret is the InfluxDB shared secret used to sign HS256 JWTs.
	// Requires: username. Conflicts with: password.
	// Corresponds to INFLUXDB_HTTP_SHARED_SECRET on the InfluxDB server.
	configKeySharedSecret = "shared_secret"

	// jwtExpiry is the lifetime of each generated JWT. Tokens are created
	// per-request so this only needs to outlive the HTTP round-trip plus any
	// clock skew between the autoscaler and the InfluxDB server.
	jwtExpiry = 2 * time.Minute
)

// influxClaims are the JWT claims expected by InfluxDB 1.x shared-secret auth.
// InfluxDB only validates exp and username; the remaining fields from
// jwt.RegisteredClaims are omitted from the token payload via omitempty.
type influxClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// setAuthHeader applies the appropriate authentication method to the HTTP
// request based on the plugin configuration:
//   - shared_secret + username → auto-generated HS256 JWT Bearer token
//   - username + password      → HTTP Basic auth
//   - neither                  → unauthenticated (local/dev scenarios)
func (a *APMPlugin) setAuthHeader(req *http.Request) error {
	if a.cfg.SharedSecret != "" {
		token, err := a.generateJWT()
		if err != nil {
			return fmt.Errorf("generating JWT: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	if a.cfg.Username != "" || a.cfg.Password != "" {
		req.SetBasicAuth(a.cfg.Username, a.cfg.Password)
	}
	return nil
}

// generateJWT creates a new HS256-signed JWT for InfluxDB 1.x Bearer auth.
// The payload contains the configured username and an expiry of now+jwtExpiry.
//
// A new token is generated on every call with no caching. This is intentional:
// unlike OAuth2/OIDC or cloud-provider token flows (which require a network
// round-trip to an external token endpoint to refresh), InfluxDB JWTs are
// signed locally using HMAC-SHA256 — a pure CPU operation that completes in
// under a microsecond. Caching would add mutex complexity with no benefit.
func (a *APMPlugin) generateJWT() (string, error) {
	expiry := time.Now().Add(jwtExpiry)
	claims := influxClaims{
		Username: a.cfg.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(a.cfg.SharedSecret))
}
