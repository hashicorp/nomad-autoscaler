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

	// configKeyTokenTTL is the optional lifetime for auto-generated JWTs.
	// Accepts Go duration strings (e.g. "30m", "2h"). Default: 1h.
	// Range: 1m – 24h. Only used when shared_secret is set.
	configKeyTokenTTL = "token_ttl"

	// defaultTokenTTL is the JWT lifetime when token_ttl is not configured.
	defaultTokenTTL = time.Hour

	// minTokenTTL / maxTokenTTL are the allowed bounds for token_ttl.
	minTokenTTL = 10 * time.Minute
	maxTokenTTL = 24 * time.Hour

	// jwtRefreshBuffer is how long before expiry a new JWT is generated.
	// Must be less than minTokenTTL to ensure the token is always cached.
	jwtRefreshBuffer = 2 * time.Minute
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
		token, err := a.getOrRefreshJWT()
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

// getOrRefreshJWT returns the cached JWT if it is still fresh, or generates
// and caches a new one. The token is refreshed jwtRefreshBuffer before expiry,
// ensuring it remains valid for at least one full evaluation cycle.
func (a *APMPlugin) getOrRefreshJWT() (string, error) {
	a.jwtMu.Lock()
	defer a.jwtMu.Unlock()

	if a.cachedToken != "" && time.Until(a.tokenExpiry) > jwtRefreshBuffer {
		return a.cachedToken, nil
	}

	token, expiry, err := a.generateJWT()
	if err != nil {
		return "", err
	}

	a.cachedToken = token
	a.tokenExpiry = expiry
	a.logger.Debug("generated new InfluxDB JWT", "expires_at", expiry.UTC().Format(time.RFC3339))
	return token, nil
}

// generateJWT creates a new HS256-signed JWT for InfluxDB 1.x Bearer auth.
// The payload contains the configured username and an expiry of now+TokenTTL.
func (a *APMPlugin) generateJWT() (string, time.Time, error) {
	expiry := time.Now().Add(a.cfg.TokenTTL)
	claims := influxClaims{
		Username: a.cfg.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(a.cfg.SharedSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiry, nil
}
