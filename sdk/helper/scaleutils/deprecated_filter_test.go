package scaleutils

import (
	"errors"
	"testing"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestPoolIdentifier_Validate(t *testing.T) {
	testCases := []struct {
		inputPID       *PoolIdentifier
		expectedOutput error
		name           string
	}{
		{
			inputPID: &PoolIdentifier{
				IdentifierKey: IdentifierKeyClass,
				Value:         "someclass",
			},
			expectedOutput: nil,
			name:           "class pool identifier",
		},
		{
			inputPID: &PoolIdentifier{
				IdentifierKey: "divination",
				Value:         "someclass",
			},
			expectedOutput: errors.New(`unsupported node pool identifier: "divination"`),
			name:           "unsupported pool identifier",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputPID.Validate(), tc.name)
		})
	}
}

func Test_filterByClass(t *testing.T) {
	testCases := []struct {
		inputNodeList       []*api.NodeListStub
		inputID             string
		expectedOutputList  []*api.NodeListStub
		expectedOutputError error
		name                string
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
			expectedOutputError: nil,
			name:                "filter of multiclass input for named class without error",
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
			expectedOutputError: nil,
			name:                "filter of multiclass input for default class without error",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{
					ID:                    "node1",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node2",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusInit,
				},
			},
			inputID:            "lionel",
			expectedOutputList: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New("node node2 is initializing")},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "filter of single class input for named class with initializing error",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{
					ID:                    "node1",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node2",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
			},
			inputID:            "lionel",
			expectedOutputList: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New("node node2 is draining")},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "filter of single class input for named class with draining error",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{
					ID:                    "node1",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node2",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
			},
			inputID:            "lionel",
			expectedOutputList: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New("node node2 is ineligible")},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "filter of single class input for named class with ineligible error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, actualError := filterByClass(tc.inputNodeList, tc.inputID)
			assert.Equal(t, tc.expectedOutputList, actualOutput, tc.name)

			if tc.expectedOutputError == nil {
				assert.Nil(t, actualError, tc.name)
			} else {
				assert.Equal(t, tc.expectedOutputError.Error(), actualError.Error(), tc.name)
			}
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
