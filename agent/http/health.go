// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"net/http"
	"sync/atomic"
)

// getHealth is the HTTP handler used to respond when a request is made to the
// health endpoint. The response is based on the aliveness parameter within the
// httpServer struct.
func (s *Server) getHealth(_ http.ResponseWriter, r *http.Request) (interface{}, error) {

	// Only allow GET requests on this endpoint.
	if r.Method != http.MethodGet {
		return nil, newCodedError(http.StatusMethodNotAllowed, errInvalidMethod)
	}

	if atomic.LoadInt32(&s.aliveness) != healthAlivenessReady {
		return nil, newCodedError(http.StatusServiceUnavailable, "Service unavailable")

	}
	return nil, nil
}
