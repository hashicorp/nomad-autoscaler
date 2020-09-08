package strategy

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type Strategy interface {
	Run(req sdk.StrategyRunReq) (sdk.ScalingAction, error)
	PluginInfo() (*base.PluginInfo, error)
	SetConfig(config map[string]string) error
}

func (s *RPCServer) PluginInfo(_ interface{}, r *base.PluginInfo) error {
	resp, err := s.Impl.PluginInfo()
	if resp != nil {
		*r = *resp
	}
	return err
}

func (r *RPC) PluginInfo() (*base.PluginInfo, error) {
	var resp base.PluginInfo
	err := r.client.Call("Plugin.PluginInfo", new(interface{}), &resp)
	if err != nil {
		return &resp, err
	}
	return &resp, nil
}

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

func (r *RPC) Run(req sdk.StrategyRunReq) (sdk.ScalingAction, error) {
	var resp sdk.ScalingAction
	err := r.client.Call("Plugin.Run", req, &resp)
	if err != nil {
		return sdk.ScalingAction{}, err
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

func (s *RPCServer) Run(req sdk.StrategyRunReq, resp *sdk.ScalingAction) error {
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
