// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

var (
	testFrom  = time.Date(2024, 5, 29, 12, 0, 0, 0, time.UTC)
	testTo    = testFrom.Add(5 * time.Minute)
	testQuery = `{"plugin":"host","metrics":["cpu.used"]}`
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

// TestAPMPlugin_QueryMultiple verifies that QueryMultiple sends the correct
// HTTP request (method, path, auth header, content-type, injected timeFrame)
// and correctly handles success responses, empty results, HTTP 429, and
// generic HTTP errors.
func TestAPMPlugin_QueryMultiple(t *testing.T) {
	testCases := []struct {
		name        string
		fixture     string            // response body loaded from test-fixtures/
		body        string            // raw response body (used when fixture is empty)
		statusCode  int               // 0 means error before any HTTP call
		respHeaders map[string]string // extra response headers
		checkBody   bool              // when true, validate injected timeFrame in request
		query       string
		timeRange   sdk.TimeRange
		expectErr   string
		expectLen   int
	}{
		{
			name:      "instant query returns error before HTTP",
			query:     testQuery,
			timeRange: sdk.TimeRange{From: testFrom, To: testFrom},
			expectErr: "instant",
		},
		{
			name:      "invalid JSON query returns error before HTTP",
			query:     "{bad-json",
			timeRange: sdk.TimeRange{From: testFrom, To: testTo},
			expectErr: "failed to unmarshal instana query",
		},
		{
			name:       "two entities returned",
			fixture:    "query_200.json",
			statusCode: http.StatusOK,
			checkBody:  true,
			query:      testQuery,
			timeRange:  sdk.TimeRange{From: testFrom, To: testTo},
			expectLen:  2,
		},
		{
			name:       "empty items returns empty result",
			fixture:    "query_empty.json",
			statusCode: http.StatusOK,
			query:      testQuery,
			timeRange:  sdk.TimeRange{From: testFrom, To: testTo},
			expectLen:  0,
		},
		{
			name:       "HTTP 500 returns error with status code",
			body:       "internal server error",
			statusCode: http.StatusInternalServerError,
			query:      testQuery,
			timeRange:  sdk.TimeRange{From: testFrom, To: testTo},
			expectErr:  "instana query failed with status 500",
		},
		{
			name:        "HTTP 429 returns rate limit error",
			statusCode:  http.StatusTooManyRequests,
			respHeaders: map[string]string{rateLimitResetHdr: "1717000999000"},
			query:       testQuery,
			timeRange:   sdk.TimeRange{From: testFrom, To: testTo},
			expectErr:   "ratelimited",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Cases with statusCode == 0 fail before making any HTTP request.
			if tc.statusCode == 0 {
				p := newTestPlugin(t, "https://fake.instana.io", "tok")
				_, err := p.QueryMultiple(tc.query, tc.timeRange)
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
				return
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				must.Eq(t, http.MethodPost, r.Method)
				must.Eq(t, metricsPath, r.URL.Path)
				must.Eq(t, "apiToken test-token", r.Header.Get("Authorization"))
				must.Eq(t, "application/json", r.Header.Get("Content-Type"))

				if tc.checkBody {
					var req instanaQueryRequest
					must.NoError(t, json.NewDecoder(r.Body).Decode(&req))
					must.Eq(t, testTo.UnixMilli(), req.TimeFrame.To)
					must.Eq(t, testTo.Sub(testFrom).Milliseconds(), req.TimeFrame.WindowSize)
				}

				for k, v := range tc.respHeaders {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.statusCode)

				switch {
				case tc.fixture != "":
					data, err := os.ReadFile(filepath.Join("test-fixtures", tc.fixture))
					must.NoError(t, err)
					_, _ = w.Write(data)
				case tc.body != "":
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer srv.Close()

			p := newTestPlugin(t, srv.URL, "test-token")
			got, err := p.QueryMultiple(tc.query, tc.timeRange)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
			} else {
				must.NoError(t, err)
				must.Len(t, tc.expectLen, got)
			}
		})
	}
}

// newTestPlugin builds an APMPlugin wired to baseURL with the given token,
// bypassing SetConfig so tests control the exact URL (e.g. an httptest server).
func newTestPlugin(t *testing.T, baseURL, token string) *APMPlugin {
	t.Helper()
	u, err := url.Parse(baseURL)
	must.NoError(t, err)
	return &APMPlugin{
		logger: hclog.NewNullLogger(),
		cfg:    pluginConfig{BaseURL: u, APIToken: token},
		client: &http.Client{},
	}
}

// TestAPMPlugin_parseItems verifies that parseItems correctly converts
// Instana response items into sdk.TimestampedMetrics slices, including
// zero-value points, multiple metrics per entity, and empty input.
func TestAPMPlugin_parseItems(t *testing.T) {
	testCases := []struct {
		name      string
		input     []instanaMetricItem
		expectLen int   // number of TimestampedMetrics slices returned
		expectPts []int // number of points in each slice (index-matched)
	}{
		{
			name:      "nil input returns empty result",
			input:     nil,
			expectLen: 0,
		},
		{
			name:      "empty items returns empty result",
			input:     []instanaMetricItem{},
			expectLen: 0,
		},
		{
			name: "single entity single metric three points",
			input: []instanaMetricItem{
				{
					SnapshotID: "abc123",
					Metrics: map[string][][2]float64{
						"cpu.used": {
							{1717000000000, 42.5},
							{1717000060000, 44.1},
							{1717000120000, 43.7},
						},
					},
				},
			},
			expectLen: 1,
			expectPts: []int{3},
		},
		{
			name: "zero value point is included not skipped",
			input: []instanaMetricItem{
				{
					SnapshotID: "abc123",
					Metrics: map[string][][2]float64{
						"cpu.used": {
							{1717000000000, 42.5},
							{1717000060000, 0},
							{1717000120000, 43.7},
						},
					},
				},
			},
			expectLen: 1,
			expectPts: []int{3},
		},
		{
			name: "two entities one metric each returns two streams",
			input: []instanaMetricItem{
				{
					SnapshotID: "abc123",
					Metrics:    map[string][][2]float64{"cpu.used": {{1717000000000, 42.5}}},
				},
				{
					SnapshotID: "def456",
					Metrics:    map[string][][2]float64{"cpu.used": {{1717000000000, 71.2}}},
				},
			},
			expectLen: 2,
			expectPts: []int{1, 1},
		},
		{
			name: "one entity two metrics returns two streams",
			input: []instanaMetricItem{
				{
					SnapshotID: "abc123",
					Metrics: map[string][][2]float64{
						"cpu.used": {{1717000000000, 42.5}, {1717000060000, 44.1}},
						"mem.used": {{1717000000000, 70.0}, {1717000060000, 72.3}},
					},
				},
			},
			expectLen: 2,
			expectPts: []int{2, 2},
		},
		{
			name: "timestamps converted from unix milliseconds correctly",
			input: []instanaMetricItem{
				{
					SnapshotID: "abc123",
					Metrics: map[string][][2]float64{
						"cpu.used": {{1717000000000, 10.0}},
					},
				},
			},
			expectLen: 1,
			expectPts: []int{1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseItems(tc.input)

			must.Len(t, tc.expectLen, got)

			for i, wantPts := range tc.expectPts {
				must.Len(t, wantPts, got[i])
			}

			// Verify the timestamp conversion for the millisecond case.
			if tc.name == "timestamps converted from unix milliseconds correctly" {
				must.Eq(t, int64(1717000000000), got[0][0].Timestamp.UnixMilli())
				must.Eq(t, 10.0, got[0][0].Value)
			}
		})
	}
}

// TestAPMPlugin_Query verifies the single-stream enforcement logic in Query:
// 0 streams → empty result, 1 stream → unwrapped, n>1 streams → error.
func TestAPMPlugin_Query(t *testing.T) {
	testCases := []struct {
		name      string
		fixture   string
		query     string
		timeRange sdk.TimeRange
		expectErr string
		expectEmpty bool // true when an empty (zero-length) result is expected
		expectLen int  // expected number of points in the single returned stream
	}{
		{
			name:      "instant query propagates error from QueryMultiple",
			query:     testQuery,
			timeRange: sdk.TimeRange{From: testFrom, To: testFrom},
			expectErr: "instant",
		},
		{
			name:      "zero streams returns empty result",
			fixture:   "query_empty.json",
			query:     testQuery,
			timeRange: sdk.TimeRange{From: testFrom, To: testTo},
			expectEmpty: true,
		},
		{
			name:      "one stream is unwrapped and returned",
			fixture:   "query_null_values.json",
			query:     testQuery,
			timeRange: sdk.TimeRange{From: testFrom, To: testTo},
			expectLen: 3,
		},
		{
			name:      "multiple streams returns error",
			fixture:   "query_200.json",
			query:     testQuery,
			timeRange: sdk.TimeRange{From: testFrom, To: testTo},
			expectErr: "only 1 is expected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Instant case fails before any HTTP call.
			if tc.fixture == "" {
				p := newTestPlugin(t, "https://fake.instana.io", "tok")
				_, err := p.Query(tc.query, tc.timeRange)
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
				return
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := os.ReadFile(filepath.Join("test-fixtures", tc.fixture))
				must.NoError(t, err)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			}))
			defer srv.Close()

			p := newTestPlugin(t, srv.URL, "test-token")
			got, err := p.Query(tc.query, tc.timeRange)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
				return
			}

			must.NoError(t, err)
			if tc.expectEmpty {
				must.Len(t, 0, got)
			} else {
				must.Len(t, tc.expectLen, got)
			}
		})
	}
}
