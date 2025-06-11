// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package http

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
)

const (
	// healthRoutePattern is the Autoscaler HTTP router pattern which is used
	// to register the health server endpoint.
	healthRoutePattern = "/v1/health"

	// metricsRoutePattern is the Autoscaler HTTP router pattern which is used
	// to register the metrics server endpoint.
	metricsRoutePattern = "/v1/metrics"

	// agentRoutePattern is the Autoscaler HTTP router pattern which is used to
	// register endpoints related to the agent.
	agentRoutePattern = "/v1/agent/"

	// healthAliveness is used to define the health of the Autoscaler agent. It
	// currently can only be in two states; ready or unavailable and depends
	// entirely on whether the server is serving or not.
	healthAlivenessReady = iota
	healthAlivenessUnavailable
)

// AgentHTTP is the interface that defines the HTTP handlers that an Agent
// must implement in order to be accessible through the HTTP API.
type AgentHTTP interface {
	// DisplayMetrics returns a summary of metrics collected by the agent.
	DisplayMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error)

	// ReloadAgent triggers the agent to reload policies and configuration.
	ReloadAgent(resp http.ResponseWriter, req *http.Request) (interface{}, error)
}

type Server struct {
	log hclog.Logger
	ln  net.Listener
	mux *http.ServeMux
	srv *http.Server

	// promEnabled tracks whether Prometheus formatted metrics should be
	// enabled.
	promEnabled bool

	// aliveness is used to describe the health response and should be set
	// atomically using healthAlivenessReady and healthAlivenessUnavailable
	// const declarations.
	aliveness int32

	// agent is the reference to an object that implements the AgentHTTP
	// interface to handle agent requests.
	agent AgentHTTP
}

// NewHTTPServer creates a new agent HTTP server.
func NewHTTPServer(debug, prom bool, cfg *config.HTTP, log hclog.Logger, agent AgentHTTP) (*Server, error) {

	srv := &Server{
		log:         log.Named("http_server"),
		mux:         http.NewServeMux(),
		agent:       agent,
		promEnabled: prom,
	}

	// Setup our handlers.
	srv.mux.HandleFunc(healthRoutePattern, srv.wrap(srv.getHealth))
	srv.mux.HandleFunc(metricsRoutePattern, srv.wrap(srv.getMetrics))
	srv.mux.HandleFunc(agentRoutePattern, srv.wrap(srv.agentSpecificRequest))

	// Setup the debugging endpoints.
	if debug {
		srv.mux.HandleFunc("/debug/pprof/", pprof.Index)
		srv.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		srv.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		srv.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		srv.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// Configure the HTTP server to the most basic level.
	srv.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%v", cfg.BindAddress, cfg.BindPort),
		Handler:      srv.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Announce on the configured network address. If there is an error in the
	// configured HTTP bind parameters, it will be caught here and the error
	// passed up to the agent.
	ln, err := net.Listen("tcp", srv.srv.Addr)
	if err != nil {
		return nil, fmt.Errorf("could not setup HTTP listener: %v", err)
	}
	srv.ln = ln

	return srv, nil
}

// Run is used to serve the HTTP server. The function will block and should be
// run via a go-routine. Unless http.Server.Serve panics/fails, the server can
// be stopped by calling the Stop function.
func (s *Server) Start() {
	s.log.Info("server now listening for connections", "address", s.srv.Addr)

	// Set our aliveness to ready.
	atomic.StoreInt32(&s.aliveness, healthAlivenessReady)

	// Call serve, checking whether the error return is the one we expect. If
	// we do get an unexpected error, set our aliveness as unavailable.
	if err := s.srv.Serve(s.ln); err != nil && err != http.ErrServerClosed {
		atomic.StoreInt32(&s.aliveness, healthAlivenessUnavailable)
		s.log.Error("failed to serve HTTP", "addr", s.srv.Addr, "error", err)
	}
}

// Stop attempts to gracefully stop the HTTP server. If the server does not
// stop before the timeout is reached, it will be ungracefully stopped.
func (s *Server) Stop() {

	// Set the health as unavailable.
	atomic.StoreInt32(&s.aliveness, healthAlivenessUnavailable)

	// Setup a context to use when calling server shutdown. 5 second timeout
	// should be plenty here, but it would be worth revisiting once we enhance
	// the health route or any other endpoints.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The server is shutting down, disable keepalive.
	s.srv.SetKeepAlivesEnabled(false)

	// Shut it down.
	if err := s.srv.Shutdown(ctx); err != nil {
		s.log.Error("could not gracefully shutdown HTTP server", "error", err)
	}
}

// wrap is a helper for all HTTP handler functions providing common
// functionality including logging and error handling.
func (s *Server) wrap(handler func(w http.ResponseWriter, r *http.Request) (interface{}, error)) func(w http.ResponseWriter, r *http.Request) {
	f := func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		// Defer a function which allows us to log the time taken to fulfill
		// the HTTP request.
		defer func() {
			s.log.Trace("request complete", "method", r.Method,
				"path", r.URL, "duration", time.Since(start))
		}()

		// Handle the request, allowing us to the get response object and any
		// error from the endpoint.
		obj, err := handler(w, r)
		if err != nil {
			s.handleHTTPError(w, r, err)
			return
		}

		// If we have a response object, encode it.
		if obj != nil {
			var buf bytes.Buffer

			enc := codec.NewEncoder(&buf, &codec.JsonHandle{HTMLCharsAsIs: true})

			// Encode the object. If we fail to do this, handle the error so
			// that this can be passed to the operator.
			err := enc.Encode(obj)
			if err != nil {
				s.handleHTTPError(w, r, err)
				return
			}

			//  Set the content type header and write the data to the HTTP
			//  reply.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buf.Bytes())
		}
	}

	return f
}

// handleHTTPError is used to handle HTTP handler errors within the wrap func.
// It sets response headers where required and ensure appropriate errors are
// logged.
func (s *Server) handleHTTPError(w http.ResponseWriter, r *http.Request, err error) {

	// Start with a default internal server error and the error message
	// that was returned.
	code := http.StatusInternalServerError
	errMsg := err.Error()

	// If the error was a custom codedError update the response code to
	// that of the wrapped error.
	if codedErr, ok := err.(codedError); ok {
		code = codedErr.Code()
	}

	// Write the status code header.
	w.WriteHeader(code)

	// Write the response body. If we get an error, log this as it will
	// provide some operator insight if this happens regularly.
	if _, wErr := w.Write([]byte(errMsg)); wErr != nil {
		s.log.Error("failed to write response error", "error", wErr)
	}
	s.log.Error("request failed", "method", r.Method, "path", r.URL, "error", errMsg, "code", code)
}
