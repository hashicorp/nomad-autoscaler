// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package strategy

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	sharedProto "github.com/hashicorp/nomad-autoscaler/plugins/shared/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// pluginClient is the gRPC client implementation of the Strategy interface.
type pluginClient struct {

	// Embed the base plugin client so that the Strategy plugin implements the
	// base interface.
	*base.PluginClient

	client  proto.StrategyPluginServiceClient
	doneCTX context.Context
}

// Run is the gRPC client implementation of the Strategy.Run interface
// function.
func (p *pluginClient) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {

	resp, err := p.client.Run(p.doneCTX, &proto.RunRequest{
		Action:            &sharedProto.ScalingAction{},
		Count:             count,
		Check:             shared.ScalingPolicyCheckToProto(eval.Check),
		TimestampedMetric: shared.TimestampedMetricsToProto(eval.Metrics),
	})
	if err != nil {
		return nil, err
	}

	action, err := shared.ProtoToScalingAction(resp.GetAction())
	if err != nil {
		return nil, err
	}

	// Update the eval with the new action and return.
	eval.Action = &action
	return eval, nil
}
