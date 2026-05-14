// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestAPMPlugin_SetConfig(t *testing.T) {
	// avoid picking up stale values from the local or CI environment
	for _, ev := range []string{envVarAddress, envVarDatabase, envVarUsername, envVarPassword, envVarSharedSecret} {
		t.Setenv(ev, "")
	}

	testCases := []struct {
		name         string
		inputConfig  map[string]string
		expectOutput error
	}{
		{
			name:         "no required config parameters set",
			inputConfig:  map[string]string{},
			expectOutput: errors.New(`"address" config value cannot be empty`),
		},
		{
			name:         "missing database",
			inputConfig:  map[string]string{configKeyAddress: "http://localhost:8086"},
			expectOutput: errors.New(`"database" config value cannot be empty`),
		},
		{
			name: "unsupported version 2",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "2",
			},
			expectOutput: errors.New(`influxdb version "2" is not yet supported: only version "1" is currently implemented`),
		},
		{
			name: "unsupported version 3",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "3",
			},
			expectOutput: errors.New(`influxdb version "3" is not yet supported: only version "1" is currently implemented`),
		},
		{
			name: "invalid version",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
				configKeyVersion:  "invalid",
			},
			expectOutput: errors.New(`invalid influxdb version "invalid": only version "1" is supported`),
		},
		{
			name: "all required config parameters set by database",
			inputConfig: map[string]string{
				configKeyAddress:  "http://localhost:8086",
				configKeyDatabase: "telegraf",
			},
			expectOutput: nil,
		},
		{
			name: "all required config parameters set by db",
			inputConfig: map[string]string{
				configKeyAddress: "http://localhost:8086",
				configKeyDB:      "telegraf",
			},
			expectOutput: nil,
		},
		{
			name: "invalid URL",
			inputConfig: map[string]string{
				configKeyAddress:  "not-a-valid-url",
				configKeyDatabase: "telegraf",
			},
			expectOutput: errors.New(`"address" must be a valid absolute URL`),
		},
		{
			name: "shared_secret with username is valid",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "my-secret",
			},
			expectOutput: nil,
		},
		{
			name: "shared_secret without username is invalid",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeySharedSecret: "my-secret",
			},
			expectOutput: errors.New(`auth configuration error: "shared_secret" requires "username" (used as the JWT username claim)`),
		},
		{
			name: "shared_secret conflicts with password",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeyPassword:     "hunter2",
				configKeySharedSecret: "my-secret",
			},
			expectOutput: errors.New(`conflicting auth configuration: "shared_secret" cannot be used together with "password"`),
		},
		{
			name: "custom token_ttl valid",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "my-secret",
				configKeyTokenTTL:     "30m",
			},
			expectOutput: nil,
		},
		{
			name: "token_ttl below minimum",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "my-secret",
				configKeyTokenTTL:     "30s",
			},
			expectOutput: errors.New(`invalid "token_ttl" value "30s": must be between 1m0s and 24h0m0s`),
		},
		{
			name: "token_ttl above maximum",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "my-secret",
				configKeyTokenTTL:     "25h",
			},
			expectOutput: errors.New(`invalid "token_ttl" value "25h": must be between 1m0s and 24h0m0s`),
		},
		{
			name: "token_ttl invalid string",
			inputConfig: map[string]string{
				configKeyAddress:      "http://localhost:8086",
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "my-secret",
				configKeyTokenTTL:     "forever",
			},
			expectOutput: errors.New(`invalid "token_ttl" value "forever"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apmPlugin := APMPlugin{logger: hclog.NewNullLogger()}

			actualOutput := apmPlugin.SetConfig(tc.inputConfig)

			if tc.expectOutput == nil {
				test.NoError(t, actualOutput)
			} else {
				must.Error(t, actualOutput)
				test.StrContains(t, actualOutput.Error(), tc.expectOutput.Error())
			}

			if tc.expectOutput == nil {
				test.NotNil(t, apmPlugin.client)
				test.NotNil(t, apmPlugin.baseURL)
			} else {
				test.Nil(t, apmPlugin.client)
				test.Nil(t, apmPlugin.baseURL)
			}
		})
	}
}

// TestAPMPlugin_SetConfig_EnvFallback tests env var credential fallback:
// config map wins when both are set, empty env var still fails validation.
func TestAPMPlugin_SetConfig_EnvFallback(t *testing.T) {
	t.Run("address from env var", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://influxdb-env:8086")
		t.Setenv(envVarDatabase, "telegraf-env")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{}))
		must.Eq(t, "http://influxdb-env:8086", p.baseURL.String())
	})

	t.Run("database from env var", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "telegraf-env")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{}))
		must.Eq(t, "telegraf-env", p.config[configKeyDatabase])
	})

	t.Run("config map overrides address env var", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://env-host:8086")
		t.Setenv(envVarDatabase, "telegraf")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{
			configKeyAddress:  "http://config-host:8086",
			configKeyDatabase: "telegraf",
		}))
		must.Eq(t, "http://config-host:8086", p.baseURL.String())
	})

	t.Run("config map overrides database env var", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "env-db")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{
			configKeyAddress:  "http://localhost:8086",
			configKeyDatabase: "config-db",
		}))
		must.Eq(t, "config-db", p.config[configKeyDatabase])
	})

	t.Run("db alias takes priority over database env var", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "env-db")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{
			configKeyAddress: "http://localhost:8086",
			configKeyDB:      "alias-db",
		}))
		// db alias should win over the env var; resolved into configKeyDatabase
		must.Eq(t, "alias-db", p.config[configKeyDatabase])
	})

	t.Run("username and password from env vars", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "telegraf")
		t.Setenv(envVarUsername, "env-user")
		t.Setenv(envVarPassword, "env-pass")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{}))
		must.Eq(t, "env-user", p.config[configKeyUsername])
		must.Eq(t, "env-pass", p.config[configKeyPassword])
	})

	t.Run("shared_secret and username from env vars", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "telegraf")
		t.Setenv(envVarUsername, "env-user")
		t.Setenv(envVarSharedSecret, "env-secret")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		must.NoError(t, p.SetConfig(map[string]string{}))
		must.Eq(t, "env-user", p.config[configKeyUsername])
		must.Eq(t, "env-secret", p.config[configKeySharedSecret])
	})

	t.Run("shared_secret from env requires username", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "telegraf")
		t.Setenv(envVarSharedSecret, "env-secret")
		// no username in config or env
		p := APMPlugin{logger: hclog.NewNullLogger()}
		err := p.SetConfig(map[string]string{})
		must.Error(t, err)
		test.StrContains(t, err.Error(), `"shared_secret" requires "username"`)
	})

	t.Run("empty address env var still fails required check", func(t *testing.T) {
		t.Setenv(envVarAddress, "")
		t.Setenv(envVarDatabase, "telegraf")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		err := p.SetConfig(map[string]string{})
		must.Error(t, err)
		test.StrContains(t, err.Error(), `"address" config value cannot be empty`)
	})

	t.Run("empty database env var still fails required check", func(t *testing.T) {
		t.Setenv(envVarAddress, "http://localhost:8086")
		t.Setenv(envVarDatabase, "")
		p := APMPlugin{logger: hclog.NewNullLogger()}
		err := p.SetConfig(map[string]string{})
		must.Error(t, err)
		test.StrContains(t, err.Error(), `"database" config value cannot be empty`)
	})
}

// TestAPMPlugin_Query tests the HTTP query path against a local test server.
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
			fixture: "query_200.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
				configKeyUsername: "user",
				configKeyPassword: "pass",
			},
			query: "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 10m",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				must.Eq(t, "/query", r.URL.Path)
				qp := r.URL.Query()
				must.Eq(t, "telegraf", qp.Get("db"))
				must.Eq(t, "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 10m", qp.Get("q"))
				must.Eq(t, "s", qp.Get("epoch"))
				// credentials must not leak into query params
				must.Eq(t, "", qp.Get("u"))
				must.Eq(t, "", qp.Get("p"))
				// Verify Basic auth header is set
				authHeader := r.Header.Get("Authorization")
				must.NotEq(t, "", authHeader)
				must.Eq(t, "Basic dXNlcjpwYXNz", authHeader) // base64(user:pass)
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				must.NoError(t, err)
				must.Len(t, 3, m)
			},
		},
		{
			name:    "handle null values",
			fixture: "query_null_result.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT mean(usage_idle) FROM cpu",
			timeRange: sdk.TimeRange{
				From: time.Unix(1660000000, 0),
				To:   time.Unix(1670000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				// Verify no auth header when credentials not provided
				must.Eq(t, "", r.Header.Get("Authorization"))
				// Verify no credentials in query params
				must.Eq(t, "", r.URL.Query().Get("u"))
				must.Eq(t, "", r.URL.Query().Get("p"))
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				must.NoError(t, err)
				must.Len(t, 2, m)
			},
		},
		{
			name:    "shared_secret JWT auth",
			fixture: "query_200.json",
			pluginConfig: map[string]string{
				configKeyDatabase:     "telegraf",
				configKeyUsername:     "autoscaler",
				configKeySharedSecret: "test-shared-secret",
			},
			query: "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 10m",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				must.NotEq(t, "", authHeader)
				must.True(t, strings.HasPrefix(authHeader, "Bearer "), must.Sprintf("expected Bearer scheme, got: %s", authHeader))

				// check the JWT and its claims
				rawToken := strings.TrimPrefix(authHeader, "Bearer ")
				tok, err := jwtlib.Parse(rawToken, func(jwtTok *jwtlib.Token) (any, error) {
					_, ok := jwtTok.Method.(*jwtlib.SigningMethodHMAC)
					must.True(t, ok, must.Sprint("expected HS256 signing method"))
					return []byte("test-shared-secret"), nil
				})
				must.NoError(t, err)
				must.True(t, tok.Valid)

				claims, ok := tok.Claims.(jwtlib.MapClaims)
				must.True(t, ok)
				must.Eq(t, "autoscaler", claims["username"])

				// exp must be in the future.
				exp, err := claims.GetExpirationTime()
				must.NoError(t, err)
				must.True(t, exp.After(time.Now()), must.Sprint("JWT exp should be in the future"))

				// No credentials in query params.
				qp := r.URL.Query()
				must.Eq(t, "", qp.Get("u"))
				must.Eq(t, "", qp.Get("p"))
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				must.NoError(t, err)
				must.Len(t, 3, m)
			},
		},
		{
			name:    "multiple streams returns error",
			fixture: "query_multiple_streams.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT usage_idle FROM cpu",
			timeRange: sdk.TimeRange{
				From: time.Unix(1660000000, 0),
				To:   time.Unix(1670000000, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				must.Error(t, err)
				must.StrContains(t, err.Error(), "only 1 is expected")
			},
		},
		{
			name:    "query memory with mean aggregation",
			fixture: "query_memory.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query: "SELECT mean(used_percent) FROM mem WHERE time > now() - 5m",
			timeRange: sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1600000200, 0),
			},
			validateMetrics: func(t *testing.T, m sdk.TimestampedMetrics, err error) {
				must.NoError(t, err)
				must.Len(t, 4, m)
				// Verify values are parsed correctly from "mean" column
				must.Eq(t, 75.5, m[0].Value)
				must.Eq(t, 78.2, m[1].Value)
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

			plugin := NewInfluxDBPlugin(hclog.NewNullLogger())
			err := plugin.SetConfig(map[string]string{
				configKeyAddress:      srv.URL,
				configKeyDatabase:     tc.pluginConfig[configKeyDatabase],
				configKeyUsername:     tc.pluginConfig[configKeyUsername],
				configKeyPassword:     tc.pluginConfig[configKeyPassword],
				configKeySharedSecret: tc.pluginConfig[configKeySharedSecret],
			})
			must.NoError(t, err)

			metrics, err := plugin.Query(tc.query, tc.timeRange)
			if tc.validateMetrics != nil {
				tc.validateMetrics(t, metrics, err)
			}
		})
	}
}

// TestAPMPlugin_JWT_Caching tests that tokens are reused within the TTL
// and refreshed once they enter the expiry window.
func TestAPMPlugin_JWT_Caching(t *testing.T) {
	p := &APMPlugin{
		logger: hclog.NewNullLogger(),
		config: map[string]string{
			configKeyUsername:     "autoscaler",
			configKeySharedSecret: "cache-test-secret",
		},
		tokenTTL: time.Hour,
	}

	// First call — token is generated.
	tok1, err := p.getOrRefreshJWT()
	must.NoError(t, err)
	must.NotEq(t, "", tok1)

	// Second call within TTL — same token returned (cached).
	tok2, err := p.getOrRefreshJWT()
	must.NoError(t, err)
	must.Eq(t, tok1, tok2, must.Sprint("expected cached token to be reused"))

	// Sleep 1s so the next token gets a different exp. JWT timestamps are
	// whole seconds, so same-second regeneration produces an identical string.
	time.Sleep(time.Second)

	// Simulate expiry by rewinding tokenExpiry into the refresh window.
	p.jwtMu.Lock()
	p.tokenExpiry = time.Now().Add(10 * time.Second) // inside 30s refresh window
	p.jwtMu.Unlock()

	// Third call — token must be refreshed with a new exp.
	tok3, err := p.getOrRefreshJWT()
	must.NoError(t, err)
	must.NotEq(t, "", tok3)
	// exp is recomputed on each generation, so a fresh token is always a different string
	must.NotEq(t, tok1, tok3, must.Sprint("expected a newly generated token after entering refresh window"))
	// make sure the new token is actually valid
	parsed, err := jwtlib.Parse(tok3, func(jwtTok *jwtlib.Token) (any, error) {
		return []byte("cache-test-secret"), nil
	})
	must.NoError(t, err)
	must.True(t, parsed.Valid)
}

func TestAPMPlugin_Query_InstantNotSupported(t *testing.T) {
	plugin := NewInfluxDBPlugin(hclog.NewNullLogger())
	err := plugin.SetConfig(map[string]string{
		configKeyAddress:  "http://localhost:8086",
		configKeyDatabase: "telegraf",
	})
	must.NoError(t, err)

	now := time.Now().UTC()
	_, err = plugin.Query("SELECT mean(usage_idle) FROM cpu", sdk.TimeRange{From: now, To: now})
	must.Error(t, err)
	must.StrContains(t, err.Error(), `query_window = "instant" is not supported by influxdb`)
}

// TestAPMPlugin_Query_Errors tests HTTP errors, InfluxDB query errors, and empty results.
func TestAPMPlugin_Query_Errors(t *testing.T) {
	testCases := []struct {
		name         string
		fixture      string
		statusCode   int
		pluginConfig map[string]string
		query        string
		expectError  string
	}{
		{
			name:       "http error response",
			statusCode: 500,
			fixture:    "query_200.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query:       "SELECT mean(usage_idle) FROM cpu",
			expectError: "influxdb query failed with status 500",
		},
		{
			name:       "influxdb level error",
			statusCode: 200,
			fixture:    "query_error.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query:       "SELECT mean(usage_idle) FROM cpu",
			expectError: "influxdb query error: database not found: telegraf_unknown",
		},
		{
			name:       "empty result",
			statusCode: 200,
			fixture:    "query_empty.json",
			pluginConfig: map[string]string{
				configKeyDatabase: "telegraf",
			},
			query:       "SELECT mean(usage_idle) FROM cpu WHERE time > now() - 1s",
			expectError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.statusCode != 200 {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte("Internal Server Error"))
					return
				}
				http.ServeFile(w, r, path.Join("./test-fixtures", tc.fixture))
			}))
			defer srv.Close()

			plugin := NewInfluxDBPlugin(hclog.NewNullLogger())
			err := plugin.SetConfig(map[string]string{
				configKeyAddress:  srv.URL,
				configKeyDatabase: tc.pluginConfig[configKeyDatabase],
			})
			must.NoError(t, err)

			metrics, err := plugin.Query(tc.query, sdk.TimeRange{
				From: time.Unix(1600000000, 0),
				To:   time.Unix(1610000000, 0),
			})

			if tc.expectError != "" {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.expectError)
			} else {
				must.NoError(t, err)
				must.Len(t, 0, metrics)
			}
		})
	}
}
