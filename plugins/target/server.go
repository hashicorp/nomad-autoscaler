// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package target

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/plugins/target/proto/v1"
)

// pluginServer is the gRPC server implementation of the Target interface.
type pluginServer struct {
	broker *plugin.GRPCBroker
	impl   Target
}

// Scale is the gRPC server implementation of the Target.Scale interface
// function.
func (p *pluginServer) Scale(_ context.Context, req *proto.ScaleRequest) (*proto.ScaleResponse, error) {
	action, err := shared.ProtoToScalingAction(req.GetAction())
	if err != nil {
		return nil, err
	}
	return &proto.ScaleResponse{}, p.impl.Scale(action, req.GetConfig())
}

// Status is the gRPC server implementation of the Target.Status interface
// function.
func (p *pluginServer) Status(_ context.Context, req *proto.StatusRequest) (*proto.StatusResponse, error) {

	statusResp, err := p.impl.Status(req.GetConfig())
	if err != nil {
		return nil, err
	}

	return &proto.StatusResponse{
		Ready: statusResp.Ready,
		Count: statusResp.Count,
		Meta:  statusResp.Meta,
	}, nil
}
