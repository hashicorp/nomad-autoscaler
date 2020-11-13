package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	AgentRequestTypeReload = iota
)

type AgentRequest struct {
	Type       int
	Request    *http.Request
	ResponseCh chan interface{}
}

// sendAgentRequest wraps an AgentRequest into a synchronous request that will
// timeout if the agent doesn't reply back in time.
func (s *Server) sendAgentRequest(req AgentRequest) interface{} {
	timeout := time.NewTimer(15 * time.Second)
	timeoutErr := fmt.Errorf("request timeout")

	if req.ResponseCh == nil {
		req.ResponseCh = make(chan interface{})
	}

	select {
	case <-timeout.C:
		return timeoutErr
	case s.agentCh <- req:
	}

	select {
	case <-timeout.C:
		return timeoutErr
	case agentResp := <-req.ResponseCh:
		return agentResp
	}
}

func (s *Server) agentSpecificRequest(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/agent")
	switch {
	case strings.HasSuffix(path, "/reload"):
		return s.agentReload(w, r)
	default:
		return nil, newCodedError(http.StatusNotFound, "")
	}
}

func (s *Server) agentReload(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return nil, newCodedError(http.StatusMethodNotAllowed, errInvalidMethod)
	}

	agentResp := s.sendAgentRequest(AgentRequest{
		Type:    AgentRequestTypeReload,
		Request: r,
	})

	if err, ok := agentResp.(error); ok && err != nil {
		return nil, newCodedError(http.StatusInternalServerError, err.Error())
	}

	return nil, nil
}
