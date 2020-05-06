package target

import (
	"net/rpc"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
)

type Target interface {
	Scale(action strategy.Action, config map[string]string) error
	Status(config map[string]string) (*Status, error)
	PluginInfo() (*base.PluginInfo, error)
	SetConfig(config map[string]string) error
}

type Status struct {
	Ready bool
	Count int64
	Meta  map[string]string
}

// MetaKeyLastEvent is an optional meta key that can be added to the status
// return. The value represents the last scaling event of the target as seen by
// the remote providers view point. This helps enforce cooldown where
// out-of-band scaling activities have been triggered.
const MetaKeyLastEvent = "nomad_autoscaler.last_event"

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

func (r *RPC) PluginInfo() (*base.PluginInfo, error) {
	var resp base.PluginInfo
	err := r.client.Call("Plugin.PluginInfo", new(interface{}), &resp)
	return &resp, err
}

func (r *RPC) Status(config map[string]string) (*Status, error) {
	var resp Status
	err := r.client.Call("Plugin.Status", config, &resp)
	return &resp, err
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

func (s *RPCServer) PluginInfo(_ interface{}, r *base.PluginInfo) error {
	resp, err := s.Impl.PluginInfo()
	if resp != nil {
		*r = *resp
	}
	return err
}

func (s *RPCServer) Status(config map[string]string, resp *Status) error {
	status, err := s.Impl.Status(config)
	if status != nil {
		*resp = *status
	}
	return err
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
