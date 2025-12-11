// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package apm

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
)

// pluginServer is the gRPC server implementation of the APM interface.
type pluginServer struct {
	broker *plugin.GRPCBroker
	impl   APM
}

// Query is the gRPC server implementation of the APM.Query interface function.
func (p *pluginServer) Query(_ context.Context, req *proto.QueryRequest) (*proto.QueryResponse, error) {

	tr, err := shared.ProtoToTimeRange(req.GetTimeRange())
	if err != nil {
		return nil, err
	}

	res, err := p.impl.Query(req.GetQuery(), *tr)
	if err != nil {
		return nil, err
	}

	return &proto.QueryResponse{
		TimestampedMetric: shared.TimestampedMetricsToProto(res),
	}, nil
}

// QueryMultiple is the gRPC client implementation of the APM.QueryMultiple
// interface function.
func (p *pluginServer) QueryMultiple(_ context.Context, req *proto.QueryMultipleRequest) (*proto.QueryMultipleResponse, error) {

	tr, err := shared.ProtoToTimeRange(req.GetTimeRange())
	if err != nil {
		return nil, err
	}

	res, err := p.impl.QueryMultiple(req.GetQuery(), *tr)
	if err != nil {
		return nil, err
	}

	out := make([]*proto.QueryResponse, len(res))

	for i, m := range res {
		out[i] = &proto.QueryResponse{TimestampedMetric: shared.TimestampedMetricsToProto(m)}
	}

	return &proto.QueryMultipleResponse{
		TimestampedMetric: out,
	}, nil
}
