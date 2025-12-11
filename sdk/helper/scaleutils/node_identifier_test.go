// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"errors"
	"testing"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_FilterNodes(t *testing.T) {
	testCases := []struct {
		inputNodeList       []*api.NodeListStub
		inputIDCfg          map[string]string
		expectedOutputNodes []*api.NodeListStub
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
			inputIDCfg: map[string]string{"node_class": "super-test-class"},
			expectedOutputNodes: []*api.NodeListStub{
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
			inputIDCfg: map[string]string{"node_class": "autoscaler-default-pool"},
			expectedOutputNodes: []*api.NodeListStub{
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
					ID:                    "node-ready-eligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node-ready-ineligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node-down-eligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusDown,
				},
				{
					ID:                    "node-down-ineligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusDown,
				},
			},
			inputIDCfg: map[string]string{"node_class": "lionel"},
			expectedOutputNodes: []*api.NodeListStub{
				{
					ID:                    "node-ready-eligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingEligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node-ready-ineligible",
					NodeClass:             "lionel",
					Drain:                 false,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
			},
			expectedOutputError: nil,
			name:                "filter of nodes that are ready",
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
			inputIDCfg:          map[string]string{"node_class": "lionel"},
			expectedOutputNodes: nil,
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
			inputIDCfg:          map[string]string{"node_class": "lionel"},
			expectedOutputNodes: nil,
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
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node2",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node3",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node4",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node5",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node6",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node7",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node8",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node9",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node10",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
				{
					ID:                    "node11",
					NodeClass:             "lionel",
					Drain:                 true,
					SchedulingEligibility: api.NodeSchedulingIneligible,
					Status:                api.NodeStatusReady,
				},
			},
			inputIDCfg:          map[string]string{"node_class": "lionel"},
			expectedOutputNodes: nil,
			expectedOutputError: &multierror.Error{
				Errors: []error{
					errors.New("node node1 is draining"),
					errors.New("node node2 is draining"),
					errors.New("node node3 is draining"),
					errors.New("node node4 is draining"),
					errors.New("node node5 is draining"),
					errors.New("node node6 is draining"),
					errors.New("node node7 is draining"),
					errors.New("node node8 is draining"),
					errors.New("node node9 is draining"),
					errors.New("node node10 is draining"),
				},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "limited error count return",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			idFn, err := nodepool.NewClusterNodePoolIdentifier(tc.inputIDCfg)
			assert.NotNil(t, idFn, tc.name)
			assert.Nil(t, err, tc.name)

			actualNodes, actualError := FilterNodes(tc.inputNodeList, idFn.IsPoolMember)
			assert.Equal(t, tc.expectedOutputNodes, actualNodes, tc.name)

			if tc.expectedOutputError != nil {
				assert.EqualError(t, actualError, tc.expectedOutputError.Error(), tc.name)
			}
		})
	}
}

func Test_filterOutNodeID(t *testing.T) {
	testCases := []struct {
		inputNodeList  []*api.NodeListStub
		inputID        string
		expectedOutput []*api.NodeListStub
		name           string
	}{
		{
			inputNodeList: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo2"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			inputID: "",
			expectedOutput: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo2"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			name: "empty input ID",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo2"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			inputID: "foo2",
			expectedOutput: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			name: "input ID found",
		},
		{
			inputNodeList: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo2"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			inputID: "bar1",
			expectedOutput: []*api.NodeListStub{
				{ID: "foo1"},
				{ID: "foo2"},
				{ID: "foo3"},
				{ID: "foo4"},
				{ID: "foo5"},
			},
			name: "input ID not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := filterOutNodeID(tc.inputNodeList, tc.inputID)
			assert.ElementsMatch(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
