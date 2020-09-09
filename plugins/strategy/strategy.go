package strategy

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type Strategy interface {

	// Run triggers a run of the strategy calculation. It is responsible for
	// populating the sdk.ScalingAction object within the passed eval and
	// returning the eval to the caller. The count input variable represents
	// the current state of the scaling target.
	Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error)

	PluginInfo() (*base.PluginInfo, error)
	SetConfig(config map[string]string) error
}

// RunRPCReq is an internal request object used by the Run function that ties
// together the two input variables as a single object as needed when calling
// the RPCServer.
type RunRPCReq struct {
	Eval  *sdk.ScalingCheckEvaluation
	Count int64
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

func (r *RPC) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	var resp sdk.ScalingCheckEvaluation
	req := RunRPCReq{
		Eval:  eval,
		Count: count,
	}
	err := r.client.Call("Plugin.Run", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

type RPCServer struct {
	Impl Strategy
}

func (s *RPCServer) SetConfig(config map[string]string, resp *error) error {
	err := s.Impl.SetConfig(config)
	*resp = err
	return err
}

func (s *RPCServer) Run(req RunRPCReq, resp *sdk.ScalingCheckEvaluation) error {
	r, err := s.Impl.Run(req.Eval, req.Count)
	if err != nil {
		return err
	}
	*resp = *r
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
