package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
)

const (
	// healthRoutePattern is the Autoscaler HTTP router pattern which is used
	// to register the health server endpoint.
	healthRoutePattern = "/v1/health"

	// healthAliveness is used to define the health of the Autoscaler agent. It
	// currently can only be in two states; ready or unavailable and depends
	// entirely on whether the server is serving or not.
	healthAlivenessReady = iota
	healthAlivenessUnavailable
)

type healthServer struct {

	// aliveness is used to describe the health response and should be set
	// atomically using healthAliveness* const declarations.
	aliveness int32

	log hclog.Logger
	srv *http.Server
	ln  net.Listener
}

// newHealthServer creates a new basic HTTP health server with a single route
// for responding to requests.
//
// TODO(jrasell) enhance this endpoint to provide more than just aliveness
//  checks.
func newHealthServer(cfg *config.HTTP, log hclog.Logger) (*healthServer, error) {

	srv := &healthServer{
		log: log.Named("health_server"),
	}

	// Setup our router and single health check route.
	router := http.NewServeMux()
	router.Handle(healthRoutePattern, srv.getHealth())

	// Configure the HTTP server to the most basic level.
	srv.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%v", cfg.BindAddress, cfg.BindPort),
		Handler:      router,
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

// run is used to serve the HTTP server. The function will block and should be
// run via a go-routine. Unless http.Server.Serve panics/fails, the server can
// be stopped by calling the stop() function.
func (hs *healthServer) run() {
	hs.log.Info("server now listening for connections", "address", hs.srv.Addr)

	// Set our aliveness to ready.
	atomic.StoreInt32(&hs.aliveness, healthAlivenessReady)

	// Call serve, checking whether the error return is the one we expect. If
	// we do get an unexpected error, set our aliveness as unavailable.
	if err := hs.srv.Serve(hs.ln); err != nil && err != http.ErrServerClosed {
		atomic.StoreInt32(&hs.aliveness, healthAlivenessUnavailable)
		hs.log.Error("failed to serve HTTP", "addr", hs.srv.Addr, "error", err)
	}
}

// stop attempts to gracefully stop the HTTP server used to serve the getHealth
// endpoint. If the server does not stop before the timeout is reached, it will
// be ungracefully stopped.
func (hs *healthServer) stop() {

	// Set the health as unavailable.
	atomic.StoreInt32(&hs.aliveness, healthAlivenessUnavailable)

	// Setup a context to use when calling server shutdown. 5 second timeout
	// should be plenty here, but it would be worth revisiting once we enhance
	// the health route.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The server is shutting down, disable keepalive.
	hs.srv.SetKeepAlivesEnabled(false)

	// Shut it down.
	if err := hs.srv.Shutdown(ctx); err != nil {
		hs.log.Error("could not gracefully shutdown HTTP server", "error", err)
	}
}

// getHealth is the HTTP handler used to respond when a request is made to the
// health endpoint. The response is based on the aliveness parameter within the
// healthServer struct.
func (hs *healthServer) getHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Only allow GET requests on this endpoint.
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if atomic.LoadInt32(&hs.aliveness) == healthAlivenessReady {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}
