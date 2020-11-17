package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_getMetrics(t *testing.T) {
	testCases := []struct {
		inputReq             *http.Request
		inputWriter          *httptest.ResponseRecorder
		expectedRespCode     int
		expectedRespContains string
		name                 string
	}{
		{
			inputReq:             httptest.NewRequest("PUT", "/v1/metrics", nil),
			inputWriter:          httptest.NewRecorder(),
			expectedRespCode:     405,
			expectedRespContains: "Invalid method",
			name:                 "incorrect request method",
		},
		{
			inputReq:             httptest.NewRequest("GET", "/v1/metrics", nil),
			inputWriter:          httptest.NewRecorder(),
			expectedRespCode:     200,
			expectedRespContains: "Counters\":[],\"Gauges\":[],\"Points\":[],\"Samples\":[]",
			name:                 "correct request for JSON metrics",
		},

		{
			inputReq:             httptest.NewRequest("GET", "/v1/metrics?format=prometheus", nil),
			inputWriter:          httptest.NewRecorder(),
			expectedRespCode:     200,
			expectedRespContains: "# TYPE go_goroutines gauge",
			name:                 "correct request for Prometheus formatted metrics",
		},
	}

	// Create our HTTP server.
	srv, stopSrv := TestServer(t)
	defer stopSrv()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.mux.ServeHTTP(tc.inputWriter, tc.inputReq)
			assert.Equal(t, tc.expectedRespCode, tc.inputWriter.Code, tc.name)
			assert.Contains(t, tc.inputWriter.Body.String(), tc.expectedRespContains, tc.name)
		})
	}
}
