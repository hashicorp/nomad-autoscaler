// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_agentReload(t *testing.T) {
	testCases := []struct {
		inputReq         *http.Request
		expectedRespCode int
		name             string
	}{
		{
			inputReq:         httptest.NewRequest("PUT", "/v1/agent/reload", nil),
			expectedRespCode: 200,
			name:             "successfully reload",
		},
		{
			inputReq:         httptest.NewRequest("GET", "/v1/agent/reload", nil),
			expectedRespCode: 405,
			name:             "incorrect request method",
		},
	}

	srv, stopSrv := TestServer(t, false)
	defer stopSrv()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, tc.inputReq)
			assert.Equal(tc.expectedRespCode, w.Code)
		})
	}
}
