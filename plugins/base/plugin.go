// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"google.golang.org/grpc"
)

// PluginBase is the Base implementation of the go-plugin GRPCPlugin interface.
type PluginBase struct {

	// Embedded so we disable support for net/rpc based plugins.
	plugin.NetRPCUnsupportedPlugin

	// Impl is the Base interface implementation that the plugin serves.
	Impl Base
}

// GRPCServer is the Base implementation of the go-plugin GRPCPlugin.GRPCServer
// interface function.
func (p *PluginBase) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterBasePluginServiceServer(s, &pluginServer{impl: p.Impl, broker: broker})
	return nil
}

// GRPCClient is the Base implementation of the go-plugin GRPCPlugin.GRPCClient
// interface function.
func (p *PluginBase) GRPCClient(ctx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &PluginClient{
		Client:  proto.NewBasePluginServiceClient(c),
		DoneCtx: ctx,
	}, nil
}
