// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_getHealth(t *testing.T) {
	testCases := []struct {
		inputReq          *http.Request
		inputWriter       *httptest.ResponseRecorder
		inputSetAliveness int32
		expectedRespCode  int
		name              string
	}{
		{
			inputReq:          httptest.NewRequest("GET", "/v1/health", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessReady,
			expectedRespCode:  200,
			name:              "agent alive and ready",
		},
		{
			inputReq:          httptest.NewRequest("GET", "/v1/health", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessUnavailable,
			expectedRespCode:  503,
			name:              "agent unavailable",
		},
		{
			inputReq:          httptest.NewRequest("PUT", "/v1/health", nil),
			inputWriter:       httptest.NewRecorder(),
			inputSetAliveness: healthAlivenessReady,
			expectedRespCode:  405,
			name:              "incorrect request method",
		},
	}

	// Create our HTTP server.
	srv, stopSrv := TestServer(t, false)
	defer stopSrv()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atomic.StoreInt32(&srv.aliveness, tc.inputSetAliveness)
			srv.mux.ServeHTTP(tc.inputWriter, tc.inputReq)
			assert.Equal(t, tc.expectedRespCode, tc.inputWriter.Code, tc.name)
		})
	}
}
