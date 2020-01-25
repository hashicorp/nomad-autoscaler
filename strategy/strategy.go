package strategy

import (
	"log"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

type Strategy interface {
	Run(req *RunRequest) ([]Action, error)
}

type RunRequest struct {
	CurrentCount int
	MinCount     int
	MaxCount     int
	CurrentValue float64
	Config       map[string]string
}

type Action struct {
	Count  int
	Reason string
}

type RPCPlugin struct {
	client *rpc.Client
}

func (r *RPCPlugin) Run(req *RunRequest) error {
	return r.client.Call("Plugin.Run", req, nil)
}

type RPCPluginServer struct {
	Impl Strategy
}

func (s *RPCPluginServer) Run(req *RunRequest, resp interface{}) error {
	_, err := s.Impl.Run(req)
	if err != nil {
		log.Printf("failed to run strategy: %v", err)
		return err
	}
	return nil
}

// Plugin is the plugin.Plugin
type Plugin struct {
	Impl Strategy
}

func (p *Plugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCPluginServer{Impl: p.Impl}, nil
}
func (Plugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPCPlugin{client: c}, nil
}
