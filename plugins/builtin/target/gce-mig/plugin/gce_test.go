package plugin

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_gceNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError error
		name                string
	}{
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone":            "us-central1-f",
					"unique.platform.gce.hostname": "instance-1.c.project.internal",
				},
			},
			expectedOutputID:    "zones/us-central1-f/instances/instance-1",
			expectedOutputError: nil,
			name:                "required attributes found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone":            "us-central1-f",
					"unique.platform.gce.hostname": "instance-1",
				},
			},
			expectedOutputID:    "zones/us-central1-f/instances/instance-1",
			expectedOutputError: nil,
			name:                "required attributes found with non-split hostname",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "platform.gce.zone" not found`),
			name:                "required attribute zone not found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone": "us-central1-f",
				},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.platform.gce.hostname" not found`),
			name:                "required attribute hostname not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := gceNodeIDMap(tc.inputNode)
			assert.Equal(t, tc.expectedOutputID, actualID, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}
