package agent

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/stretchr/testify/assert"
)

func Test_healthServer_health(t *testing.T) {
	testCases := []struct {
		inputReq          *http.Request
		inputWriter       *httptest.ResponseRecorder
		inputSetAliveness int32
		expectedRespCode  int
	}{
		{
			inputReq:          httptest.NewRequest("GET", "http://localhost:8080/v1/getHealth", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessReady,
			expectedRespCode:  200,
		},
		{
			inputReq:          httptest.NewRequest("GET", "http://localhost:8080/v1/getHealth", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessUnavailable,
			expectedRespCode:  503,
		},
		{
			inputReq:          httptest.NewRequest("PUT", "http://localhost:8080/v1/getHealth", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessReady,
			expectedRespCode:  405,
		},
	}

	svr, err := newHealthServer(&config.HTTP{BindAddress: "localhost", BindPort: 8080}, hclog.NewNullLogger())
	assert.Nil(t, err)

	for _, tc := range testCases {
		atomic.StoreInt32(&svr.aliveness, tc.inputSetAliveness)
		svr.getHealth().ServeHTTP(tc.inputWriter, tc.inputReq)
		assert.Equal(t, tc.expectedRespCode, tc.inputWriter.Code)
	}
}
