// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package target

import (
	"context"
	"time"

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
	ctx := p.doneCTX
	if timeoutString, ok := config[shared.PluginConfigKeyGRPCTimeout]; ok {
		timeout, err := time.ParseDuration(timeoutString)
		if err != nil {
			return nil, err
		}

		var cancel func()
		ctx, cancel = context.WithTimeout(p.doneCTX, timeout)
		defer cancel()
	}

	statusResp, err := p.client.Status(ctx, &proto.StatusRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return &sdk.TargetStatus{
		Ready: statusResp.Ready,
		Count: statusResp.Count,
		Meta:  statusResp.Meta,
	}, nil
}
