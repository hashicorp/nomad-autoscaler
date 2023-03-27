// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestServer_handlerHTTPError(t *testing.T) {
	testCases := []struct {
		inputReq         *http.Request
		inputWriter      *httptest.ResponseRecorder
		inputError       error
		expectedRespCode int
		expectedRespBody string
		name             string
	}{
		{
			inputReq:         httptest.NewRequest("GET", "/v1/health", nil),
			inputWriter:      httptest.NewRecorder(),
			inputError:       errors.New("random error string"),
			expectedRespCode: 500,
			expectedRespBody: "random error string",
			name:             "internal server error",
		},
		{
			inputReq:         httptest.NewRequest("GET", "/v1/health", nil),
			inputWriter:      httptest.NewRecorder(),
			inputError:       newCodedError(418, "I'm a teapot"),
			expectedRespCode: 418,
			expectedRespBody: "I'm a teapot",
			name:             "custom error using codedError",
		},
	}

	srv := &Server{log: hclog.NewNullLogger()}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv.handleHTTPError(tc.inputWriter, tc.inputReq, tc.inputError)
			assert.Equal(t, tc.expectedRespCode, tc.inputWriter.Code, tc.name)
			assert.Equal(t, tc.expectedRespBody, tc.inputWriter.Body.String(), tc.name)
		})
	}
}
