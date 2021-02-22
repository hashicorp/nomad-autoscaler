package plugin

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

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
