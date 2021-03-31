package nodeselector

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_defaultClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "newest_create_index", newNewestCreateIndexClusterScaleInNodeSelector().Name())
}

func Test_defaultClusterScaleInNodeSelector_Select(t *testing.T) {
	testCases := []struct {
		inputNodes     []*api.NodeListStub
		inputNum       int
		expectedOutput []*api.NodeListStub
		name           string
	}{
		{
			inputNodes: []*api.NodeListStub{
				{ID: "8d01d8fe-3c6b-0fdd-ebbc-a8b5cbb9c00c"},
				{ID: "a6604683-fd65-c913-77ee-a0a15b8e88e9"},
			},
			inputNum: 1,
			expectedOutput: []*api.NodeListStub{
				{ID: "8d01d8fe-3c6b-0fdd-ebbc-a8b5cbb9c00c"},
			},
			name: "single selection needed",
		},

		{
			inputNodes: []*api.NodeListStub{
				{ID: "8d01d8fe-3c6b-0fdd-ebbc-a8b5cbb9c00c"},
				{ID: "a6604683-fd65-c913-77ee-a0a15b8e88e9"},
			},
			inputNum: 3,
			expectedOutput: []*api.NodeListStub{
				{ID: "8d01d8fe-3c6b-0fdd-ebbc-a8b5cbb9c00c"},
				{ID: "a6604683-fd65-c913-77ee-a0a15b8e88e9"},
			},
			name: "single selection needed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := newNewestCreateIndexClusterScaleInNodeSelector().Select(tc.inputNodes, tc.inputNum)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.inputNodes)
		})
	}
}
