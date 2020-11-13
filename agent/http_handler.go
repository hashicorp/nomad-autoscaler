package agent

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad-autoscaler/agent/http"
)

func (a *Agent) handleHTTPRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-a.httpServer.AgentCh():
			switch r.Type {
			case http.AgentRequestTypeReload:
				a.handleHTTPReload(r)
			default:
				r.ResponseCh <- fmt.Errorf("invalid request type %q", r.Type)
			}
		}
	}
}

func (a *Agent) handleHTTPReload(r http.AgentRequest) {
	a.reload()
	r.ResponseCh <- nil
}
