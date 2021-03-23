package plugin

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_instancesBelongToASG(t *testing.T) {
	testCases := []struct {
		inputASG            *autoscaling.AutoScalingGroup
		inputIDs            []scaleutils.NodeResourceID
		expectedOutputList  []string
		expectedOutputError error
		name                string
	}{
		{
			inputASG: &autoscaling.AutoScalingGroup{
				Instances: []autoscaling.Instance{
					{InstanceId: aws.String("i-08d2c60605d210f51")},
					{InstanceId: aws.String("i-08d2c60605d210f52")},
					{InstanceId: aws.String("i-08d2c60605d210f53")},
					{InstanceId: aws.String("i-08d2c60605d210f54")},
					{InstanceId: aws.String("i-08d2c60605d210f55")},
				},
			},
			inputIDs: []scaleutils.NodeResourceID{
				{RemoteResourceID: "i-08d2c60605d210f51"},
				{RemoteResourceID: "i-08d2c60605d210f54"},
			},
			expectedOutputList: []string{
				"i-08d2c60605d210f51",
				"i-08d2c60605d210f54",
			},
			expectedOutputError: nil,
			name:                "multiple matches with zero failure",
		},
		{
			inputASG: &autoscaling.AutoScalingGroup{
				Instances: []autoscaling.Instance{
					{InstanceId: aws.String("i-08d2c60605d210f51")},
					{InstanceId: aws.String("i-08d2c60605d210f52")},
					{InstanceId: aws.String("i-08d2c60605d210f53")},
					{InstanceId: aws.String("i-08d2c60605d210f54")},
					{InstanceId: aws.String("i-08d2c60605d210f55")},
				},
			},
			inputIDs: []scaleutils.NodeResourceID{
				{RemoteResourceID: "i-08d2c60605d210f51"},
				{RemoteResourceID: "i-08d2c60605d210f54"},
				{RemoteResourceID: "i-08d2c60605d210f58"},
			},
			expectedOutputList:  nil,
			expectedOutputError: errors.New("1 selected nodes are not found within ASG"),
			name:                "multiple matches with zero failure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualList, actualErr := instancesBelongToASG(tc.inputASG, tc.inputIDs)
			assert.Equal(t, tc.expectedOutputList, actualList, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}

func Test_awsNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError error
		name                string
	}{
		{
			inputNode: &api.Node{
				Attributes: map[string]string{"unique.platform.aws.instance-id": "i-1234567890abcdef0"},
			},
			expectedOutputID:    "i-1234567890abcdef0",
			expectedOutputError: nil,
			name:                "required attribute found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.platform.aws.instance-id" not found`),
			name:                "required attribute not found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{"unique.platform.aws.instance-id": ""},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.platform.aws.instance-id" not found`),
			name:                "required attribute found but empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := awsNodeIDMap(tc.inputNode)
			assert.Equal(t, tc.expectedOutputID, actualID, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}
