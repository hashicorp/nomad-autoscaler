package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
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

	// Create a simple in-memory sink to use.
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)

	// Create our HTTP server.
	srv, err := NewHTTPServer(&config.HTTP{BindAddress: "127.0.0.1", BindPort: 8080}, hclog.NewNullLogger(), inm)
	assert.Nil(t, err)
	defer srv.ln.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.mux.ServeHTTP(tc.inputWriter, tc.inputReq)
			assert.Equal(t, tc.expectedRespCode, tc.inputWriter.Code, tc.name)
			assert.Contains(t, tc.inputWriter.Body.String(), tc.expectedRespContains, tc.name)
		})
	}
}
