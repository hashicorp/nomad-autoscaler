// Copyright IBM Corp. 2020, 2026

package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

func Test_newInstanaClient_EndpointJoining(t *testing.T) {
	testCases := []struct {
		name      string
		endpoint  string
		expectURL string
	}{
		{
			name:      "root endpoint",
			endpoint:  "https://example.instana.io",
			expectURL: "https://example.instana.io/api/infrastructure-monitoring/metrics",
		},
		{
			name:      "root endpoint with trailing slash",
			endpoint:  "https://example.instana.io/",
			expectURL: "https://example.instana.io/api/infrastructure-monitoring/metrics",
		},
		{
			name:      "base path endpoint",
			endpoint:  "https://example.instana.io/custom-base",
			expectURL: "https://example.instana.io/custom-base/api/infrastructure-monitoring/metrics",
		},
		{
			name:      "base path endpoint with trailing slash",
			endpoint:  "https://example.instana.io/custom-base/",
			expectURL: "https://example.instana.io/custom-base/api/infrastructure-monitoring/metrics",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newInstanaClient(tc.endpoint, "test-token")
			must.NoError(t, err)
			must.Eq(t, tc.expectURL, c.metricsURL)
		})
	}
}

func Test_getInfrastructureMetrics(t *testing.T) {
	validReq := instanaMetricsRequest{
		Plugin:  "host",
		Metrics: []string{"cpu.used"},
		TimeFrame: instanaTimeFrame{
			WindowSize: testTo.Sub(testFrom).Milliseconds(),
			To:         testTo.UnixMilli(),
		},
	}

	testCases := []struct {
		name           string
		fixture        string
		statusCode     int
		rateLimitReset string
		expectItems    int
		expectErr      string
	}{
		{
			name:        "successful response returns decoded items",
			fixture:     "query_200.json",
			statusCode:  http.StatusOK,
			expectItems: 2,
		},
		{
			name:        "empty items response is valid",
			fixture:     "query_empty.json",
			statusCode:  http.StatusOK,
			expectItems: 0,
		},
		{
			name:        "response with zero-value data points decodes without error",
			fixture:     "query_zero_value.json",
			statusCode:  http.StatusOK,
			expectItems: 1,
		},
		{
			name:       "HTTP 500 returns error",
			statusCode: http.StatusInternalServerError,
			expectErr:  "instana query failed with status 500",
		},
		{
			name:           "HTTP 429 returns Instana rate limit reset error",
			statusCode:     http.StatusTooManyRequests,
			rateLimitReset: "1717000000000",
			expectErr:      "metric queries are rate limited by instana, resets at 1717000000000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// verify request shape
				must.Eq(t, http.MethodPost, r.Method)
				must.Eq(t, metricsPath, r.URL.Path)
				must.Eq(t, "apiToken test-token", r.Header.Get("Authorization"))
				must.Eq(t, "application/json", r.Header.Get("Content-Type"))

				if tc.rateLimitReset != "" {
					w.Header().Set(rateLimitResetHdr, tc.rateLimitReset)
				}
				w.WriteHeader(tc.statusCode)
				if tc.fixture != "" {
					data, err := os.ReadFile(filepath.Join("test-fixtures", tc.fixture))
					must.NoError(t, err)
					_, _ = w.Write(data)
				}
			}))
			defer srv.Close()

			c, err := newInstanaClient(srv.URL, "test-token")
			must.NoError(t, err)

			res, err := c.getInfrastructureMetrics(context.Background(), validReq)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
			} else {
				must.NoError(t, err)
				must.NotNil(t, res)
				must.Eq(t, tc.expectItems, len(res.Items))
			}
		})
	}
}
