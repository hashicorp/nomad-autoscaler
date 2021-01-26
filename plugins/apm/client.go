package apm

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/plugins/apm/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// pluginClient is the gRPC client implementation of the APM interface.
type pluginClient struct {
	*base.PluginClient
	client  proto.APMPluginServiceClient
	doneCtx context.Context
}

// Query is the gRPC client implementation of the APM.Query interface function.
func (p *pluginClient) Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error) {

	protoTS, err := shared.TimeRangeToProto(timeRange)
	if err != nil {
		return nil, err
	}

	metrics, err := p.client.Query(p.DoneCtx, &proto.QueryRequest{Query: query, TimeRange: protoTS})
	if err != nil {
		return nil, err
	}
	return shared.ProtoToTimestampedMetrics(metrics.GetTimestampedMetric()), nil
}

// QueryMultiple is the gRPC client implementation of the APM.QueryMultiple
// interface function.
func (p *pluginClient) QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {

	protoTS, err := shared.TimeRangeToProto(timeRange)
	if err != nil {
		return nil, err
	}

	metrics, err := p.client.QueryMultiple(p.DoneCtx, &proto.QueryMultipleRequest{Query: query, TimeRange: protoTS})
	if err != nil {
		return nil, err
	}

	out := make([]sdk.TimestampedMetrics, len(metrics.TimestampedMetric))

	for i, m := range metrics.TimestampedMetric {
		out[i] = shared.ProtoToTimestampedMetrics(m.GetTimestampedMetric())
	}
	return out, nil
}
