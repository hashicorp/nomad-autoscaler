package target

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/strategy"
)

// RPC is a plugin implementation that talks over net/rpc
type RPC struct {
	client *rpc.Client
}

type RPCScaleRequest struct {
	Action strategy.Action
	Config map[string]string
}

func (r *RPC) SetConfig(config map[string]string) error {
	var resp error
	err := r.client.Call("Plugin.SetConfig", config, &resp)
	if err != nil {
		return err
	}
	return resp
}

func (r *RPC) Count(config map[string]string) (int64, error) {
	var resp int64
	err := r.client.Call("Plugin.Count", config, &resp)
	if err != nil {
		return 0, err
	}
	return resp, nil
}

func (r *RPC) Scale(action strategy.Action, config map[string]string) error {
	var resp error
	req := RPCScaleRequest{
		Action: action,
		Config: config,
	}
	err := r.client.Call("Plugin.Scale", req, &resp)
	if err != nil {
		return err
	}
	return resp
}

// RPCServer is the net/rpc server
type RPCServer struct {
	Impl Target
}

func (s *RPCServer) SetConfig(config map[string]string, resp *error) error {
	err := s.Impl.SetConfig(config)
	*resp = err
	return err
}

func (s *RPCServer) Count(config map[string]string, resp *int64) error {
	count, err := s.Impl.Count(config)
	if err != nil {
		return err
	}
	*resp = count
	return nil
}

func (s *RPCServer) Scale(req RPCScaleRequest, resp *error) error {
	err := s.Impl.Scale(req.Action, req.Config)
	return err
}

// Plugin is the plugin.Plugin
type Plugin struct {
	Impl Target
}

func (p *Plugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}

func (Plugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPC{client: c}, nil
}
