// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package strategy

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	baseproto "github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy/proto/v1"
	"google.golang.org/grpc"
)

// PluginStrategy is the Strategy implementation of the go-plugin GRPCPlugin
// interface.
type PluginStrategy struct {

	// Embedded so we disable support for net/rpc based plugins.
	plugin.NetRPCUnsupportedPlugin

	// Impl is the Strategy interface implementation that the plugin serves.
	Impl Strategy
}

// GRPCServer is the Strategy implementation of the go-plugin
// GRPCPlugin.GRPCServer interface function.
func (p *PluginStrategy) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterStrategyPluginServiceServer(s, &pluginServer{impl: p.Impl, broker: broker})
	return nil
}

// GRPCClient is the Strategy implementation of the go-plugin
// GRPCPlugin.GRPCClient interface function.
func (p *PluginStrategy) GRPCClient(ctx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &pluginClient{
		PluginClient: &base.PluginClient{
			DoneCtx: ctx,
			Client:  baseproto.NewBasePluginServiceClient(c),
		},
		client:  proto.NewStrategyPluginServiceClient(c),
		doneCTX: ctx,
	}, nil
}
