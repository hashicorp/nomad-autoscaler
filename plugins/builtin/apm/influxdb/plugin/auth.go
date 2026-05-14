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
	minTokenTTL = time.Minute
	maxTokenTTL = 24 * time.Hour
)

// setAuthHeader applies the appropriate authentication method to the HTTP
// request based on the plugin configuration:
//   - shared_secret + username → auto-generated HS256 JWT Bearer token
//   - username + password      → HTTP Basic auth
//   - neither                  → unauthenticated (local/dev scenarios)
func (a *APMPlugin) setAuthHeader(req *http.Request) error {
	if a.cfg.SharedSecret != "" {
		token, err := a.getOrRefreshJWT()
		if err != nil {
			return fmt.Errorf("failed to generate JWT: %v", err)
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
// and caches a new one. The refresh window is max(30s, 10% of token_ttl),
// so a token is regenerated slightly before it would expire.
func (a *APMPlugin) getOrRefreshJWT() (string, error) {
	var refreshWindow time.Duration
	if w := a.cfg.TokenTTL / 10; w > 30*time.Second {
		refreshWindow = w
	} else {
		refreshWindow = 30 * time.Second
	}

	a.jwtMu.Lock()
	defer a.jwtMu.Unlock()

	if a.cachedToken != "" && time.Until(a.tokenExpiry) > refreshWindow {
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
// The JWT payload contains the configured username and an expiry of now+TokenTTL,
// matching the format expected by InfluxDB's shared-secret JWT authentication.
func (a *APMPlugin) generateJWT() (string, time.Time, error) {
	expiry := time.Now().Add(a.cfg.TokenTTL)
	claims := jwt.MapClaims{
		"username": a.cfg.Username,
		"exp":      expiry.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(a.cfg.SharedSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiry, nil
}
