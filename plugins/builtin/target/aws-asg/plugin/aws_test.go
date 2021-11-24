package plugin

import (
	"errors"
	"os"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_SetConfig_NonAWS(t *testing.T) {
	if os.Getenv("NOMAD_AUTOSCALER_AWS") != "" {
		t.Skip("skipping aws-asg non-aws environment test, NOMAD_AUTOSCALER_AWS set")
	}

	testCases := []struct {
		inputCfg          map[string]string
		expectedOutputErr error
		name              string
	}{
		{
			inputCfg:          map[string]string{},
			expectedOutputErr: nil,
			name:              "empty input config",
		},
		{
			inputCfg: map[string]string{
				"aws_region": "ap-southeast-2",
			},
			expectedOutputErr: nil,
			name:              "input config with region",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsPlugin := NewAWSASGPlugin(hclog.NewNullLogger())
			actualOutputErr := awsPlugin.SetConfig(tc.inputCfg)
			assert.Equal(t, tc.expectedOutputErr, actualOutputErr, tc.name)

			region := tc.inputCfg["aws_region"]
			if region == "" {
				region = "us-east-1"
			}
			assert.Equal(t, region, awsPlugin.asg.Config.Region, tc.name)
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
