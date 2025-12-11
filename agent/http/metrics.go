// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Only create the prometheus handler once
	promHandler http.Handler
	promOnce    sync.Once
)

// getMetrics is a HTTP handler which responds with the agents telemetry
// metrics data. The metrics are returned in either JSON or Prometheus
// depending and the query format.
func (s *Server) getMetrics(w http.ResponseWriter, r *http.Request) (interface{}, error) {

	// Only allow GET requests on this endpoint.
	if r.Method != http.MethodGet {
		return nil, newCodedError(http.StatusMethodNotAllowed, errInvalidMethod)
	}

	if format := r.URL.Query().Get("format"); format == "prometheus" {

		// Only return Prometheus formatted metrics if the user has enabled
		// this functionality.
		if !s.promEnabled {
			return nil, newCodedError(http.StatusUnsupportedMediaType, "Prometheus is not enabled")
		}
		s.getPrometheusMetrics().ServeHTTP(w, r)
		return nil, nil
	}
	return s.agent.DisplayMetrics(w, r)
}

// getPrometheusMetrics is the getMetrics handler when the caller wishes to
// view them in Prometheus format.
func (s *Server) getPrometheusMetrics() http.Handler {
	promOnce.Do(func() {
		handlerOptions := promhttp.HandlerOpts{
			ErrorLog:           s.log.Named("prometheus_handler").StandardLogger(nil),
			ErrorHandling:      promhttp.ContinueOnError,
			DisableCompression: true,
		}
		promHandler = promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOptions)
	})
	return promHandler
}
