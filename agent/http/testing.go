package http

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
)

func TestServer(t *testing.T) (*Server, func()) {
	cfg := &config.HTTP{
		BindAddress: "127.0.0.1",
		BindPort:    0, // Use next available port.
	}

	s, err := NewHTTPServer(cfg, hclog.NewNullLogger(), &agent.MockAgentHTTP{})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}

	return s, func() {
		s.Stop()
	}
}
