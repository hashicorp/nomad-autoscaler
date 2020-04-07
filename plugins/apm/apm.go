package apm

import (
	"net/rpc"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
)

type APM interface {
	Query(q string) (float64, error)
	PluginInfo() (*plugins.PluginInfo, error)
	SetConfig(config map[string]string) error
}

// RPC is a plugin implementation that talks over net/rpc
type RPC struct {
	client *rpc.Client
}

func (r *RPC) SetConfig(config map[string]string) error {
	var resp error
	err := r.client.Call("Plugin.SetConfig", config, &resp)
	if err != nil {
		return err
	}
	return resp
}

func (r *RPC) Query(q string) (float64, error) {
	var resp float64
	err := r.client.Call("Plugin.Query", q, &resp)
	if err != nil {
		return 0, err
	}
	return resp, nil
}

func (r *RPC) PluginInfo() (*plugins.PluginInfo, error) {
	var resp plugins.PluginInfo
	err := r.client.Call("Plugin.PluginInfo", new(interface{}), &resp)
	if err != nil {
		return &resp, err
	}
	return &resp, nil
}

// RPCServer is the net/rpc server
type RPCServer struct {
	Impl APM
}

func (s *RPCServer) SetConfig(config map[string]string, resp *error) error {
	err := s.Impl.SetConfig(config)
	*resp = err
	return err
}

func (s *RPCServer) Query(q string, resp *float64) error {
	r, err := s.Impl.Query(q)
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

func (s *RPCServer) PluginInfo(_ interface{}, r *plugins.PluginInfo) error {
	resp, err := s.Impl.PluginInfo()
	if resp != nil {
		*r = *resp
	}
	return err
}

// Plugin is the plugin.Plugin
type Plugin struct {
	Impl APM
}

func (p *Plugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}

func (Plugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPC{client: c}, nil
}
