// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package shared

import (
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"google.golang.org/protobuf/types/known/anypb"
)

// ScalingDirectionToProto converts the input scale direction to the proto
// equivalent.
func ScalingDirectionToProto(input sdk.ScaleDirection) (proto.ScalingDirection, error) {

	var out proto.ScalingDirection
	var err error

	switch input {
	case sdk.ScaleDirectionDown:
		out = proto.ScalingDirection_SCALING_DIRECTION_DOWN
	case sdk.ScaleDirectionUp:
		out = proto.ScalingDirection_SCALING_DIRECTION_UP
	case sdk.ScaleDirectionNone:
		out = proto.ScalingDirection_SCALING_DIRECTION_NONE
	default:
		out = proto.ScalingDirection_SCALING_DIRECTION_UNSPECIFIED
		err = fmt.Errorf("scale direction is unknown: %q", input)
	}

	return out, err
}

// ProtoToScalingDirection converts the input proto definition of
// ScalingDirection and returns the Autoscaler equivalent.
func ProtoToScalingDirection(input proto.ScalingDirection) (sdk.ScaleDirection, error) {

	var out sdk.ScaleDirection

	switch input {
	case proto.ScalingDirection_SCALING_DIRECTION_DOWN:
		out = sdk.ScaleDirectionDown
	case proto.ScalingDirection_SCALING_DIRECTION_UP:
		out = sdk.ScaleDirectionUp
	case proto.ScalingDirection_SCALING_DIRECTION_NONE:
		out = sdk.ScaleDirectionNone
	default:
		return out, fmt.Errorf("scale direction is unknown: %q", input)
	}

	return out, nil
}

// TimeRangeToProto takes an input time range and converts it to the proto
// equivalent.
func TimeRangeToProto(input sdk.TimeRange) (*proto.TimeRange, error) {

	toTS, err := ptypes.TimestampProto(input.To)
	if err != nil {
		return nil, err
	}

	fromTS, err := ptypes.TimestampProto(input.From)
	if err != nil {
		return nil, err
	}

	return &proto.TimeRange{
		To:   toTS,
		From: fromTS,
	}, nil
}

// ProtoToTimeRange converts the input proto definition of TimeRange and
// returns the Autoscaler equivalent.
func ProtoToTimeRange(input *proto.TimeRange) (*sdk.TimeRange, error) {

	toTS, err := ptypes.Timestamp(input.GetTo())
	if err != nil {
		return nil, err
	}

	fromTS, err := ptypes.Timestamp(input.GetFrom())
	if err != nil {
		return nil, err
	}

	return &sdk.TimeRange{
		To:   toTS,
		From: fromTS,
	}, nil
}

// TimestampedMetricsToProto converts the input TimestampedMetrics to the proto
// equivalent.
func TimestampedMetricsToProto(input sdk.TimestampedMetrics) []*proto.TimestampedMetric {

	out := make([]*proto.TimestampedMetric, len(input))

	for i, m := range input {
		ts, _ := ptypes.TimestampProto(m.Timestamp)
		out[i] = &proto.TimestampedMetric{Timestamp: ts, Value: m.Value}
	}
	return out
}

// ProtoToTimestampedMetrics converts the input proto TimestampedMetric object
// and returns the Autoscaler equivalent.
func ProtoToTimestampedMetrics(input []*proto.TimestampedMetric) sdk.TimestampedMetrics {

	out := make(sdk.TimestampedMetrics, len(input))

	for i, m := range input {
		ts, _ := ptypes.Timestamp(m.Timestamp)
		out[i] = sdk.TimestampedMetric{Timestamp: ts, Value: m.Value}
	}
	return out
}

// ActionMetaToProto converts the input meta map to the proto equivalent.
func ActionMetaToProto(input map[string]interface{}) (*anypb.Any, error) {
	if input == nil {
		return nil, nil
	}
	out, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return &anypb.Any{Value: out}, nil
}

// ProtoToActionMeta converts the input proto any object and returns the meta
// map equivalent.
func ProtoToActionMeta(input *anypb.Any) (map[string]interface{}, error) {
	if input == nil {
		return nil, nil
	}
	out := make(map[string]interface{})
	err := json.Unmarshal(input.GetValue(), &out)
	return out, err
}

// ScalingActionToProto converts the input ScalingAction to the proto
// equivalent.
func ScalingActionToProto(input sdk.ScalingAction) (*proto.ScalingAction, error) {

	dir, err := ScalingDirectionToProto(input.Direction)
	if err != nil {
		return nil, err
	}

	meta, err := ActionMetaToProto(input.Meta)
	if err != nil {
		return nil, err
	}

	return &proto.ScalingAction{
		Count:     input.Count,
		Reason:    input.Reason,
		Error:     input.Error,
		Direction: dir,
		Meta:      meta,
	}, nil
}

// ProtoToScalingAction converts the input proto ScalingAction object and
// returns the the Autoscaler equivalent.
func ProtoToScalingAction(input *proto.ScalingAction) (sdk.ScalingAction, error) {

	var out sdk.ScalingAction

	if input == nil {
		return out, nil
	}

	dir, err := ProtoToScalingDirection(input.GetDirection())
	if err != nil {
		return out, err
	}
	meta, err := ProtoToActionMeta(input.GetMeta())
	if err != nil {
		return out, err
	}

	out.Count = input.GetCount()
	out.Reason = input.GetReason()
	out.Error = input.GetError()
	out.Direction = dir
	out.Meta = meta
	out.Canonicalize()

	return out, nil
}

// ScalingPolicyCheckToProto converts the input ScalingPolicyCheck to the proto
// equivalent.
func ScalingPolicyCheckToProto(input *sdk.ScalingPolicyCheck) *proto.ScalingPolicyCheck {
	return &proto.ScalingPolicyCheck{
		Name:        input.Name,
		Source:      input.Source,
		Query:       input.Query,
		QueryWindow: ptypes.DurationProto(input.QueryWindow),
		Strategy: &proto.ScalingPolicyStrategy{
			Name:   input.Strategy.Name,
			Config: input.Strategy.Config,
		},
	}
}

// ProtoToScalingPolicyCheck converts the input proto ScalingPolicyCheck object
// and returns the the Autoscaler equivalent.
func ProtoToScalingPolicyCheck(input *proto.ScalingPolicyCheck) (*sdk.ScalingPolicyCheck, error) {

	queryWindow, err := ptypes.Duration(input.GetQueryWindow())
	if err != nil {
		return nil, err
	}

	return &sdk.ScalingPolicyCheck{
		Name:        input.GetName(),
		Source:      input.GetSource(),
		Query:       input.GetQuery(),
		QueryWindow: queryWindow,
		Strategy: &sdk.ScalingPolicyStrategy{
			Name:   input.GetStrategy().GetName(),
			Config: input.GetStrategy().GetConfig(),
		},
	}, nil
}
