package strategy

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// pluginServer is the gRPC server implementation of the Strategy interface.
type pluginServer struct {
	broker *plugin.GRPCBroker
	impl   Strategy
}

// Run is the gRPC server implementation of the Strategy.Run interface function.
func (p *pluginServer) Run(_ context.Context, req *proto.RunRequest) (*proto.RunResponse, error) {

	queryWindow, err := ptypes.Duration(req.Check.GetQueryWindow())
	if err != nil {
		return nil, err
	}

	// Populate the eval. At this point of the evaluation flow we will only
	// have Check and Metrics sections populated, so only translate this.
	eval := sdk.ScalingCheckEvaluation{
		Action: &sdk.ScalingAction{},
		Check: &sdk.ScalingPolicyCheck{
			Name:        req.Check.GetName(),
			Source:      req.Check.GetSource(),
			Query:       req.Check.GetQuery(),
			QueryWindow: queryWindow,
			Strategy: &sdk.ScalingPolicyStrategy{
				Name:   req.Check.Strategy.GetName(),
				Config: req.Check.Strategy.GetConfig(),
			},
		},
		Metrics: shared.ProtoToTimestampedMetrics(req.TimestampedMetric),
	}

	resp, err := p.impl.Run(&eval, req.GetCount())
	if err != nil {
		return nil, err
	}

	// Populate the action and re-use the request Check and metrics so we don't
	// have to incur time penalties on data which hasn't been touched or
	// changed.
	action, err := shared.ScalingActionToProto(*resp.Action)
	if err != nil {
		return nil, err
	}

	return &proto.RunResponse{
		Action:            action,
		Check:             req.GetCheck(),
		TimestampedMetric: req.GetTimestampedMetric(),
	}, nil
}
