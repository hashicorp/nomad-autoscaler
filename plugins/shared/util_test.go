package shared

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_ScalingDirectionToProto(t *testing.T) {
	testCases := []struct {
		input                   sdk.ScaleDirection
		expectedOutputDirection proto.ScalingDirection
		expectedOutputError     error
	}{
		{
			input:                   sdk.ScaleDirectionNone,
			expectedOutputDirection: proto.ScalingDirection_SCALING_DIRECTION_NONE,
			expectedOutputError:     nil,
		},
		{
			input:                   sdk.ScaleDirectionUp,
			expectedOutputDirection: proto.ScalingDirection_SCALING_DIRECTION_UP,
			expectedOutputError:     nil,
		},
		{
			input:                   sdk.ScaleDirectionDown,
			expectedOutputDirection: proto.ScalingDirection_SCALING_DIRECTION_DOWN,
			expectedOutputError:     nil,
		},
		{
			input:                   13,
			expectedOutputDirection: proto.ScalingDirection_SCALING_DIRECTION_UNSPECIFIED,
			expectedOutputError:     errors.New(`scale direction is unknown: "none"`),
		},
	}

	for _, tc := range testCases {
		actualDirection, actualError := ScalingDirectionToProto(tc.input)
		assert.Equal(t, tc.expectedOutputDirection, actualDirection)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ProtoToScalingDirection(t *testing.T) {
	testCases := []struct {
		input                   proto.ScalingDirection
		expectedOutputDirection sdk.ScaleDirection
		expectedOutputError     error
	}{
		{
			input:                   proto.ScalingDirection_SCALING_DIRECTION_DOWN,
			expectedOutputDirection: sdk.ScaleDirectionDown,
			expectedOutputError:     nil,
		},
		{
			input:                   proto.ScalingDirection_SCALING_DIRECTION_UP,
			expectedOutputDirection: sdk.ScaleDirectionUp,
			expectedOutputError:     nil,
		},
		{
			input:                   proto.ScalingDirection_SCALING_DIRECTION_NONE,
			expectedOutputDirection: sdk.ScaleDirectionNone,
			expectedOutputError:     nil,
		},
		{
			input:                   proto.ScalingDirection_SCALING_DIRECTION_UNSPECIFIED,
			expectedOutputDirection: sdk.ScaleDirectionNone,
			expectedOutputError:     errors.New(`scale direction is unknown: "SCALING_DIRECTION_UNSPECIFIED"`),
		},
	}

	for _, tc := range testCases {
		actualDirection, actualError := ProtoToScalingDirection(tc.input)
		assert.Equal(t, tc.expectedOutputDirection, actualDirection)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_TimeRangeToProto(t *testing.T) {
	testCases := []struct {
		input               sdk.TimeRange
		expectedOutputRange *proto.TimeRange
		expectedOutputError error
	}{
		{
			input: sdk.TimeRange{
				From: time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC),
				To:   time.Date(2020, time.April, 13, 9, 4, 0, 0, time.UTC),
			},
			expectedOutputRange: &proto.TimeRange{
				To:   timestamppb.New(time.Date(2020, time.April, 13, 9, 4, 0, 0, time.UTC)),
				From: timestamppb.New(time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC)),
			},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualTimeRange, actualError := TimeRangeToProto(tc.input)
		assert.Equal(t, tc.expectedOutputRange, actualTimeRange)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ProtoToTimeRange(t *testing.T) {
	testCases := []struct {
		input               *proto.TimeRange
		expectedOutputRange *sdk.TimeRange
		expectedOutputError error
	}{
		{
			input: &proto.TimeRange{
				To:   timestamppb.New(time.Date(2020, time.April, 13, 9, 4, 0, 0, time.UTC)),
				From: timestamppb.New(time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC)),
			},
			expectedOutputRange: &sdk.TimeRange{
				From: time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC),
				To:   time.Date(2020, time.April, 13, 9, 4, 0, 0, time.UTC),
			},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualTimeRange, actualError := ProtoToTimeRange(tc.input)
		assert.Equal(t, tc.expectedOutputRange, actualTimeRange)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ActionMetaToProto(t *testing.T) {
	testCases := []struct {
		input               map[string]interface{}
		expectedOutputPB    *anypb.Any
		expectedOutputError error
	}{
		{
			input: map[string]interface{}{"foo": "bar"},
			expectedOutputPB: &anypb.Any{
				Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
			},
			expectedOutputError: nil,
		},
		{
			input: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			expectedOutputPB: &anypb.Any{
				Value: mapToByteArray(t, map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}),
			},
			expectedOutputError: nil,
		},
		{
			input: map[string]interface{}{},
			expectedOutputPB: &anypb.Any{
				Value: []byte{123, 125},
			},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualPB, actualError := ActionMetaToProto(tc.input)
		assert.Equal(t, tc.expectedOutputPB, actualPB)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ProtoToActionMeta(t *testing.T) {
	testCases := []struct {
		input               *anypb.Any
		expectedOutputMap   map[string]interface{}
		expectedOutputError error
	}{
		{
			input: &anypb.Any{
				Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
			},
			expectedOutputMap:   map[string]interface{}{"foo": "bar"},
			expectedOutputError: nil,
		},
		{
			input: &anypb.Any{
				Value: mapToByteArray(t, map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}),
			},
			expectedOutputMap:   map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			expectedOutputError: nil,
		},
		{
			input: &anypb.Any{
				Value: []byte{123, 125},
			},
			expectedOutputMap:   map[string]interface{}{},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualMap, actualError := ProtoToActionMeta(tc.input)
		assert.Equal(t, tc.expectedOutputMap, actualMap)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ScalingActionToProto(t *testing.T) {
	testCases := []struct {
		input               sdk.ScalingAction
		expectedOutputProto *proto.ScalingAction
		expectedOutputError error
	}{
		{
			input: sdk.ScalingAction{
				Count:     8,
				Reason:    "because I want to",
				Error:     false,
				Direction: sdk.ScaleDirectionUp,
				Meta:      map[string]interface{}{"foo": "bar"},
			},
			expectedOutputProto: &proto.ScalingAction{
				Count:     8,
				Reason:    "because I want to",
				Error:     false,
				Direction: proto.ScalingDirection_SCALING_DIRECTION_UP,
				Meta: &anypb.Any{
					Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
				},
			},
			expectedOutputError: nil,
		},
		{
			input: sdk.ScalingAction{
				Count:     8,
				Reason:    "because I failed to",
				Error:     true,
				Direction: sdk.ScaleDirectionDown,
				Meta:      map[string]interface{}{"foo": "bar"},
			},
			expectedOutputProto: &proto.ScalingAction{
				Count:     8,
				Reason:    "because I failed to",
				Error:     true,
				Direction: proto.ScalingDirection_SCALING_DIRECTION_DOWN,
				Meta: &anypb.Any{
					Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
				},
			},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualProto, actualError := ScalingActionToProto(tc.input)
		assert.Equal(t, tc.expectedOutputProto, actualProto)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ProtoToScalingAction(t *testing.T) {
	testCases := []struct {
		input                *proto.ScalingAction
		expectedOutputAction sdk.ScalingAction
		expectedOutputError  error
	}{
		{
			input: &proto.ScalingAction{
				Count:     8,
				Reason:    "because I want to",
				Error:     false,
				Direction: proto.ScalingDirection_SCALING_DIRECTION_UP,
				Meta: &anypb.Any{
					Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
				},
			},
			expectedOutputAction: sdk.ScalingAction{
				Count:     8,
				Reason:    "because I want to",
				Error:     false,
				Direction: sdk.ScaleDirectionUp,
				Meta:      map[string]interface{}{"foo": "bar"},
			},
			expectedOutputError: nil,
		},
		{
			input: &proto.ScalingAction{
				Count:     8,
				Reason:    "because I failed to",
				Error:     true,
				Direction: proto.ScalingDirection_SCALING_DIRECTION_DOWN,
				Meta: &anypb.Any{
					Value: mapToByteArray(t, map[string]interface{}{"foo": "bar"}),
				},
			},
			expectedOutputAction: sdk.ScalingAction{
				Count:     8,
				Reason:    "because I failed to",
				Error:     true,
				Direction: sdk.ScaleDirectionDown,
				Meta:      map[string]interface{}{"foo": "bar"},
			},
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		actualAction, actualError := ProtoToScalingAction(tc.input)
		assert.Equal(t, tc.expectedOutputAction, actualAction)
		assert.Equal(t, tc.expectedOutputError, actualError)
	}
}

func Test_ScalingPolicyCheckToProto(t *testing.T) {
	testCases := []struct {
		input               *sdk.ScalingPolicyCheck
		expectedOutputCheck *proto.ScalingPolicyCheck
	}{
		{
			input: &sdk.ScalingPolicyCheck{
				Name:        "foo-bar-check",
				Source:      "prometheus",
				Query:       "some(super-weird/query).format()",
				QueryWindow: 5 * time.Second,
				Strategy: &sdk.ScalingPolicyStrategy{
					Name:   "target-value",
					Config: map[string]string{"target": "13"},
				},
			},
			expectedOutputCheck: &proto.ScalingPolicyCheck{
				Name:        "foo-bar-check",
				Source:      "prometheus",
				Query:       "some(super-weird/query).format()",
				QueryWindow: ptypes.DurationProto(5 * time.Second),
				Strategy: &proto.ScalingPolicyStrategy{
					Name:   "target-value",
					Config: map[string]string{"target": "13"},
				},
			},
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutputCheck, ScalingPolicyCheckToProto(tc.input))
	}
}

func Test_ProtoToScalingPolicyCheck(t *testing.T) {
	testCases := []struct {
		input               *proto.ScalingPolicyCheck
		expectedOutputCheck *sdk.ScalingPolicyCheck
	}{
		{
			input: &proto.ScalingPolicyCheck{
				Name:        "foo-bar-check",
				Source:      "prometheus",
				Query:       "some(super-weird/query).format()",
				QueryWindow: ptypes.DurationProto(5 * time.Second),
				Strategy: &proto.ScalingPolicyStrategy{
					Name:   "target-value",
					Config: map[string]string{"target": "13"},
				},
			},
			expectedOutputCheck: &sdk.ScalingPolicyCheck{
				Name:        "foo-bar-check",
				Source:      "prometheus",
				Query:       "some(super-weird/query).format()",
				QueryWindow: 5 * time.Second,
				Strategy: &sdk.ScalingPolicyStrategy{
					Name:   "target-value",
					Config: map[string]string{"target": "13"},
				},
			},
		},
	}

	for _, tc := range testCases {
		actualOutputCheck, outputErr := ProtoToScalingPolicyCheck(tc.input)
		assert.Equal(t, tc.expectedOutputCheck, actualOutputCheck)
		assert.Nil(t, outputErr)
	}
}

func mapToByteArray(t *testing.T, input map[string]interface{}) []byte {
	out, err := json.Marshal(input)
	if err != nil {
		t.Fail()
	}
	return out
}
