package apm

import (
	"net/rpc"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type APM interface {
	Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error)
	PluginInfo() (*base.PluginInfo, error)
	SetConfig(config map[string]string) error
}

type QueryRPCReq struct {
	Query string
	Range sdk.TimeRange
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

func (r *RPC) Query(q string, rng sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	req := QueryRPCReq{Query: q, Range: rng}
	var resp sdk.TimestampedMetrics

	err := r.client.Call("Plugin.Query", req, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (r *RPC) PluginInfo() (*base.PluginInfo, error) {
	var resp base.PluginInfo
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

func (s *RPCServer) Query(req QueryRPCReq, resp *sdk.TimestampedMetrics) error {
	r, err := s.Impl.Query(req.Query, req.Range)
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

func (s *RPCServer) PluginInfo(_ interface{}, r *base.PluginInfo) error {
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
