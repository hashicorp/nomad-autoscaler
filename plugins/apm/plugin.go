// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package apm

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	baseProto "github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"google.golang.org/grpc"
)

// PluginAPM is the APM implementation of the go-plugin GRPCPlugin interface.
type PluginAPM struct {

	// Embedded so we disable support for net/rpc based plugins.
	plugin.NetRPCUnsupportedPlugin

	// Impl is the APM interface implementation that the plugin serves.
	Impl APM
}

// GRPCServer is the APM implementation of the go-plugin GRPCPlugin.GRPCServer
// interface function.
func (p *PluginAPM) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterAPMPluginServiceServer(s, &pluginServer{impl: p.Impl, broker: broker})
	return nil
}

// GRPCClient is the APM implementation of the go-plugin GRPCPlugin.GRPCClient
// interface function.
func (p *PluginAPM) GRPCClient(ctx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &pluginClient{
		PluginClient: &base.PluginClient{
			DoneCtx: ctx,
			Client:  baseProto.NewBasePluginServiceClient(c),
		},
		client:  proto.NewAPMPluginServiceClient(c),
		doneCtx: ctx,
	}, nil
}
