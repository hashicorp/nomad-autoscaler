package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

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
		name       string
		fixture    string
		statusCode int
		expectErr  string
	}{
		{
			name:       "successful response returns decoded items",
			fixture:    "query_200.json",
			statusCode: http.StatusOK,
		},
		{
			name:       "empty items response is valid",
			fixture:    "query_empty.json",
			statusCode: http.StatusOK,
		},
		{
			name:       "response with zero-value data points decodes without error",
			fixture:    "query_null_values.json",
			statusCode: http.StatusOK,
		},
		{
			name:       "HTTP 500 returns error",
			statusCode: http.StatusInternalServerError,
			expectErr:  "instana query failed with status 500",
		},
		{
			name:       "HTTP 429 returns rate limit error",
			statusCode: http.StatusTooManyRequests,
			expectErr:  "ratelimited",
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

				w.WriteHeader(tc.statusCode)
				if tc.fixture != "" {
					data, err := os.ReadFile(filepath.Join("test-fixtures", tc.fixture))
					must.NoError(t, err)
					_, _ = w.Write(data)
				}
			}))
			defer srv.Close()

			u, _ := url.Parse(srv.URL)
			c := &instanaClient{baseURL: u, apiToken: "test-token", http: &http.Client{}}

			res, err := c.getInfrastructureMetrics(context.Background(), validReq)

			if tc.expectErr != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectErr)
			} else {
				must.NoError(t, err)
				must.NotNil(t, res)
			}
		})
	}
}
