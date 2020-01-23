package strategy

import (
	"log"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

type Strategy interface {
	Run(config map[string]interface{}) ([]StrategyResult, error)
}

type StrategyResult struct {
	JobID  string
	Count  int
	Reason string
}

type RPCPlugin struct {
	client *rpc.Client
}

func (r *RPCPlugin) Run(config map[string]interface{}) error {
	return r.client.Call("Plugin.Run", config, nil)
}

type RPCPluginServer struct {
	Impl Strategy
}

func (s *RPCPluginServer) Run(config map[string]interface{}, resp interface{}) error {
	_, err := s.Impl.Run(config)
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
