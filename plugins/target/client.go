// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package target

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/plugins/target/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// pluginClient is the gRPC client implementation of the Target interface.
type pluginClient struct {

	// Embed the base plugin client so that the Target plugin implements the
	// base interface.
	*base.PluginClient

	client  proto.TargetPluginServiceClient
	doneCTX context.Context
}

// Scale is the gRPC client implementation of the Target.Scale interface
// function.
func (p *pluginClient) Scale(action sdk.ScalingAction, config map[string]string) error {
	req, err := shared.ScalingActionToProto(action)
	if err != nil {
		return err
	}
	_, err = p.client.Scale(p.doneCTX, &proto.ScaleRequest{Action: req, Config: config})
	return err
}

// Status is the gRPC client implementation of the Target.Status interface
// function.
func (p *pluginClient) Status(config map[string]string) (*sdk.TargetStatus, error) {

	statusResp, err := p.client.Status(p.doneCTX, &proto.StatusRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return &sdk.TargetStatus{
		Ready: statusResp.Ready,
		Count: statusResp.Count,
		Meta:  statusResp.Meta,
	}, nil
}
