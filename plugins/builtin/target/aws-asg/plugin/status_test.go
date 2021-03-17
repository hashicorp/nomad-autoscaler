package plugin

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/stretchr/testify/assert"
)

func Test_instanceTerminationResult_Error(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput string
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: "",
			name:           "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
				},
			},
			expectedOutput: "failed to terminate node 711eb2aa-48cc-2dc7-32fa-b359878121cd with AWS ID i-08d2c60605d210f57: this is the error you're looking for",
			name:           "single input result error",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
							RemoteResourceID: "i-08d2c60605d210f58",
						},
						err: errors.New("this isn't the error you're looking for"),
					},
				},
			},
			expectedOutput: "failed to terminate node 711eb2aa-48cc-2dc7-32fa-b359878121cd with AWS ID i-08d2c60605d210f57: this is the error you're looking for, failed to terminate node 711eb2aa-48cc-2dc7-32fa-b359878121ce with AWS ID i-08d2c60605d210f58: this isn't the error you're looking for",
			name:           "multiple input result error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.Error(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_errorOrNil(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput error
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: nil,
			name:           "no errors",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
				},
			},
			expectedOutput: errors.New("failed to terminate node 711eb2aa-48cc-2dc7-32fa-b359878121cd with AWS ID i-08d2c60605d210f57: this is the error you're looking for"),
			name:           "error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.errorOrNil(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_activityIDs(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput []string
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: nil,
			name:           "empty result",
		},
		{
			inputResult: &instanceTerminationResult{
				success: []instanceSuccess{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459690"),
					},
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cf",
							RemoteResourceID: "i-08d2c60605d210f59",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459691"),
					},
				},
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
							RemoteResourceID: "i-08d2c60605d210f58",
						},
						err: errors.New("this is the error you're looking for"),
					},
				},
			},
			expectedOutput: []string{
				"f9f2d65b-f1f2-43e7-b46d-d86756459690",
				"f9f2d65b-f1f2-43e7-b46d-d86756459691",
			},
			name: "failed and success within result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.activityIDs(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_failedIDs(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput []scaleutils.NodeResourceID
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: nil,
			name:           "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
				},
			},
			expectedOutput: []scaleutils.NodeResourceID{
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
					RemoteResourceID: "i-08d2c60605d210f57",
				},
			},
			name: "single entry",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
							RemoteResourceID: "i-08d2c60605d210f58",
						},
						err: errors.New("this isn't the error you're looking for"),
					},
				},
			},
			expectedOutput: []scaleutils.NodeResourceID{
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
					RemoteResourceID: "i-08d2c60605d210f57",
				},
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
					RemoteResourceID: "i-08d2c60605d210f58",
				},
			},
			name: "multiple entries",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.failedIDs(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_successfulIDs(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput []scaleutils.NodeResourceID
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: nil,
			name:           "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				success: []instanceSuccess{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459690"),
					},
				},
			},
			expectedOutput: []scaleutils.NodeResourceID{
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
					RemoteResourceID: "i-08d2c60605d210f57",
				},
			},
			name: "single entry",
		},
		{
			inputResult: &instanceTerminationResult{
				success: []instanceSuccess{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459690"),
					},
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
							RemoteResourceID: "i-08d2c60605d210f58",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459691"),
					},
				},
			},
			expectedOutput: []scaleutils.NodeResourceID{
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
					RemoteResourceID: "i-08d2c60605d210f57",
				},
				{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121ce",
					RemoteResourceID: "i-08d2c60605d210f58",
				},
			},
			name: "multiple entries",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.successfulIDs(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_appendFailure(t *testing.T) {
	testCases := []struct {
		inputResult *instanceTerminationResult
		inputErr    instanceFailure
		name        string
	}{
		{
			inputResult: &instanceTerminationResult{},
			inputErr: instanceFailure{
				instance: scaleutils.NodeResourceID{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cb",
					RemoteResourceID: "i-08d2c60605d210f56",
				},
				err: errors.New("this isn't the error you're looking for"),
			},
			name: "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: []instanceFailure{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						err: errors.New("this is the error you're looking for"),
					},
				},
			},
			inputErr: instanceFailure{
				instance: scaleutils.NodeResourceID{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cb",
					RemoteResourceID: "i-08d2c60605d210f56",
				},
				err: errors.New("this isn't the error you're looking for"),
			},
			name: "non-empty input result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputResult.appendFailure(tc.inputErr)
			assert.Contains(t, tc.inputResult.failed, tc.inputErr, tc.name)
		})
	}
}

func Test_instanceTerminationResult_appendSuccess(t *testing.T) {
	testCases := []struct {
		inputResult *instanceTerminationResult
		inputInf    instanceSuccess
		name        string
	}{
		{
			inputResult: &instanceTerminationResult{},
			inputInf: instanceSuccess{
				instance: scaleutils.NodeResourceID{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cb",
					RemoteResourceID: "i-08d2c60605d210f56",
				},
				activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459699"),
			},
			name: "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				success: []instanceSuccess{
					{
						instance: scaleutils.NodeResourceID{
							NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cd",
							RemoteResourceID: "i-08d2c60605d210f57",
						},
						activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459690"),
					},
				},
			},
			inputInf: instanceSuccess{
				instance: scaleutils.NodeResourceID{
					NomadNodeID:      "711eb2aa-48cc-2dc7-32fa-b359878121cb",
					RemoteResourceID: "i-08d2c60605d210f56",
				},
				activityID: aws.String("f9f2d65b-f1f2-43e7-b46d-d86756459699"),
			},
			name: "non-empty input result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputResult.appendSuccess(tc.inputInf)
			assert.Contains(t, tc.inputResult.success, tc.inputInf, tc.name)
		})
	}
}

func Test_instanceTerminationResult_lenFailure(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput int
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: 0,
			name:           "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				failed: make([]instanceFailure, 13),
			},
			expectedOutput: 13,
			name:           "non-zero input result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.lenFailure(), tc.name)
		})
	}
}

func Test_instanceTerminationResult_lenSuccess(t *testing.T) {
	testCases := []struct {
		inputResult    *instanceTerminationResult
		expectedOutput int
		name           string
	}{
		{
			inputResult:    &instanceTerminationResult{},
			expectedOutput: 0,
			name:           "empty input result",
		},
		{
			inputResult: &instanceTerminationResult{
				success: make([]instanceSuccess, 13),
			},
			expectedOutput: 13,
			name:           "non-zero input result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, tc.inputResult.lenSuccess(), tc.name)
		})
	}
}
