package plugin

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/hashicorp/nomad/api"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/stretchr/testify/assert"
)

func Test_validateServers(t *testing.T) {
	testCases := []struct {
		inputServers        []*hcloud.Server
		inputIDs            []scaleutils.NodeResourceID
		expectedOutputError error
		name                string
	}{
		{
			inputServers: []*hcloud.Server{
				{
					Name: "test-1",
				},
				{
					Name: "test-2",
				},
				{
					Name: "test-3",
				},
				{
					Name: "test-4",
				},
			},
			inputIDs: []scaleutils.NodeResourceID{
				{RemoteResourceID: "test-1"},
				{RemoteResourceID: "test-2"},
			},
			expectedOutputError: nil,
			name:                "multiple matches with zero failure",
		},
		{
			inputServers: []*hcloud.Server{
				{
					Name: "test-1",
				},
				{
					Name: "test-2",
				},
				{
					Name: "test-3",
				},
				{
					Name: "test-4",
				},
			},
			inputIDs: []scaleutils.NodeResourceID{
				{RemoteResourceID: "test-1"},
				{RemoteResourceID: "test-5"},
			},
			expectedOutputError: errors.New("1 selected nodes are not found among Hetzner Cloud nodes"),
			name:                "multiple matches with zero failure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualErr := validateServers(tc.inputServers, tc.inputIDs)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}

func Test_hcloudNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError error
		name                string
	}{
		{
			inputNode: &api.Node{
				Attributes: map[string]string{"unique.hostname": "test-1"},
			},
			expectedOutputID:    "test-1",
			expectedOutputError: nil,
			name:                "required attribute found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.hostname" not found`),
			name:                "required attribute not found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{"unique.hostname": ""},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.hostname" not found`),
			name:                "required attribute found but empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := hcloudNodeIDMap(tc.inputNode)
			assert.Equal(t, tc.expectedOutputID, actualID, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}
