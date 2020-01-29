package strategy

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

type RPC struct {
	client *rpc.Client
}

type RunRequest struct {
	CurrentCount int
	MinCount     int
	MaxCount     int
	CurrentValue float64
	Config       map[string]string
}

type RunResponse struct {
	Actions []Action
}

func (r *RPC) SetConfig(config map[string]string) error {
	var resp error
	err := r.client.Call("Plugin.SetConfig", config, &resp)
	if err != nil {
		return err
	}
	return resp
}

func (r *RPC) Run(req RunRequest) (RunResponse, error) {
	var resp RunResponse
	err := r.client.Call("Plugin.Run", req, &resp)
	if err != nil {
		return RunResponse{}, err
	}

	return resp, nil
}

type RPCServer struct {
	Impl Strategy
}

func (s *RPCServer) SetConfig(config map[string]string, resp *error) error {
	err := s.Impl.SetConfig(config)
	*resp = err
	return err
}

func (s *RPCServer) Run(req RunRequest, resp *RunResponse) error {
	r, err := s.Impl.Run(req)
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

// Plugin is the plugin.Plugin
type Plugin struct {
	Impl Strategy
}

func (p *Plugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}
func (Plugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPC{client: c}, nil
}
