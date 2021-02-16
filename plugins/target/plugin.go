package target

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	baseproto "github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/target/proto/v1"
	"google.golang.org/grpc"
)

// PluginTarget is the Target implementation of the go-plugin GRPCPlugin
// interface.
type PluginTarget struct {

	// Embedded so we disable support for net/rpc based plugins.
	plugin.NetRPCUnsupportedPlugin

	// Impl is the Target interface implementation that the plugin serves.
	Impl Target
}

// GRPCServer is the Target implementation of the go-plugin
// GRPCPlugin.GRPCServer interface function.
func (p *PluginTarget) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterTargetPluginServiceServer(s, &pluginServer{impl: p.Impl, broker: broker})
	return nil
}

// GRPCClient is the Target implementation of the go-plugin
// GRPCPlugin.GRPCClient interface function.
func (p *PluginTarget) GRPCClient(ctx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &pluginClient{
		PluginClient: &base.PluginClient{
			DoneCtx: ctx,
			Client:  baseproto.NewBasePluginServiceClient(c),
		},
		client:  proto.NewTargetPluginServiceClient(c),
		doneCTX: ctx,
	}, nil
}
