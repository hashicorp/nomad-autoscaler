package scaleutils

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_filterByClass(t *testing.T) {
	testCases := []struct {
		inputNodeList      []*api.NodeListStub
		inputID            string
		expectedOutputList []*api.NodeListStub
		name               string
	}{
		{
			inputNodeList: []*api.NodeListStub{
				{
					ID:                    "super-test-class-1",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "super-test-class-2",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusDown,
				},
				{
					ID:                    "super-test-class-3",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "worst-test-class-1",
					NodeClass:             "worst-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "worst-test-class-2",
					NodeClass:             "worst-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "no-test-class-2",
					NodeClass:             "",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
			},
			inputID: "super-test-class",
			expectedOutputList: []*api.NodeListStub{
				{
					ID:                    "super-test-class-1",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "super-test-class-3",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
			},
			name: "complex filter of multiclass input for named class",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{
					ID:                    "super-test-class-1",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "super-test-class-2",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusDown,
				},
				{
					ID:                    "super-test-class-3",
					NodeClass:             "super-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "worst-test-class-1",
					NodeClass:             "worst-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "worst-test-class-2",
					NodeClass:             "worst-test-class",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "no-test-class-1",
					NodeClass:             "",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
			},
			inputID: "autoscaler-default-pool",
			expectedOutputList: []*api.NodeListStub{
				{
					ID:                    "no-test-class-1",
					NodeClass:             "",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
			},
			name: "complex filter of multiclass input for default class",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := filterByClass(tc.inputNodeList, tc.inputID)
			assert.Equal(t, tc.expectedOutputList, actualOutput, tc.name)
		})
	}
}

func Test_awsNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode            *api.Node
		expectedOutputString string
		expectedOutputError  error
		name                 string
	}{
		{
			inputNode: &api.Node{
				ID: "8a3025c6-5739-5563-f5e2-46000113646a",
				Attributes: map[string]string{
					"unique.platform.aws.instance-id": "i-1234567890abcdef0",
				},
			},
			expectedOutputString: "i-1234567890abcdef0",
			expectedOutputError:  nil,
			name:                 "attribute found",
		},

		{
			inputNode: &api.Node{
				ID:         "8a3025c6-5739-5563-f5e2-46000113646a",
				Attributes: map[string]string{},
			},
			expectedOutputString: "",
			expectedOutputError:  errors.New("attribute \"unique.platform.aws.instance-id\" not found"),
			name:                 "attribute not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualString, actualError := awsNodeIDMap(tc.inputNode)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
		})
	}
}
