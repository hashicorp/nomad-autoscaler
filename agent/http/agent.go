package http

import (
	"net/http"
	"strings"
)

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

	return s.agent.Reload(w, r)
}
