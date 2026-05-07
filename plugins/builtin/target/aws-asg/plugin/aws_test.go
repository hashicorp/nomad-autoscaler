// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newTestPlugin() *TargetPlugin {
	return &TargetPlugin{
		logger: hclog.NewNullLogger(),
	}
}

func asgWithName(name string) *types.AutoScalingGroup {
	return &types.AutoScalingGroup{
		AutoScalingGroupName: aws.String(name),
	}
}

func TestTargetPlugin_scaleIn_TerminateSuspended(t *testing.T) {
	testCases := []struct {
		name               string
		suspendedProcesses []types.SuspendedProcess
	}{
		{
			name: "only_terminate_suspended",
			suspendedProcesses: []types.SuspendedProcess{
				{ProcessName: aws.String("Terminate"), SuspensionReason: aws.String("manual")},
			},
		},
		{
			name: "terminate_suspended_among_others",
			suspendedProcesses: []types.SuspendedProcess{
				{ProcessName: aws.String("Launch"), SuspensionReason: aws.String("manual")},
				{ProcessName: aws.String("Terminate"), SuspensionReason: aws.String("manual")},
				{ProcessName: aws.String("HealthCheck"), SuspensionReason: aws.String("manual")},
			},
		},
		{
			name: "terminate_suspended_nil_reason",
			suspendedProcesses: []types.SuspendedProcess{
				{ProcessName: aws.String("Terminate"), SuspensionReason: nil},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tp := newTestPlugin()
			asg := asgWithName("test-asg")
			asg.SuspendedProcesses = tc.suspendedProcesses

			err := tp.scaleIn(t.Context(), asg, 1, map[string]string{})
			require.NoError(t, err, "scaleIn must return nil when Terminate is suspended")
		})
	}
}

func TestTargetPlugin_scaleIn_NonTerminateProcessesSuspended(t *testing.T) {
	tp := newTestPlugin()
	asg := asgWithName("test-asg")
	asg.SuspendedProcesses = []types.SuspendedProcess{
		{ProcessName: aws.String("Launch"), SuspensionReason: aws.String("manual")},
		{ProcessName: aws.String("HealthCheck"), SuspensionReason: aws.String("manual")},
	}
	// Set desired == minSize so the MinSize guard stops execution cleanly.
	asg.DesiredCapacity = aws.Int32(3)
	asg.MinSize = aws.Int32(3)

	err := tp.scaleIn(t.Context(), asg, 1, map[string]string{})
	require.NoError(t, err, "non-Terminate suspensions must not block scale-in logic")
}

func TestTargetPlugin_scaleIn_MinSizeGuard(t *testing.T) {
	testCases := []struct {
		name      string
		desired   int32
		minSize   int32
		num       int64
		expectNil bool
	}{
		{
			name:      "desired_equals_minSize",
			desired:   3,
			minSize:   3,
			num:       1,
			expectNil: true,
		},
		{
			name:      "desired_below_minSize",
			desired:   2,
			minSize:   3,
			num:       1,
			expectNil: true,
		},
		{
			name:      "desired_above_minSize_num_within_headroom",
			desired:   5,
			minSize:   3,
			num:       1,
			expectNil: false,
		},
		{
			name:      "desired_above_minSize_num_exceeds_headroom",
			desired:   5,
			minSize:   3,
			num:       4,
			expectNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tp := newTestPlugin()
			asg := asgWithName("test-asg")
			asg.DesiredCapacity = aws.Int32(tc.desired)
			asg.MinSize = aws.Int32(tc.minSize)

			if tc.expectNil {
				// desired <= minSize: scaleIn returns nil without proceeding.
				err := tp.scaleIn(t.Context(), asg, tc.num, map[string]string{})
				require.NoError(t, err, "scaleIn must return nil when ASG is at or below MinSize")
			} else {
				// desired > minSize: scaleIn proceeds past the MinSize guard.
				// Without a real clusterUtils this panics, which proves the
				// guard did NOT block execution — i.e. scale-in was attempted.
				require.Panics(t, func() {
					_ = tp.scaleIn(t.Context(), asg, tc.num, map[string]string{})
				}, "scaleIn must proceed past MinSize guard when desired > minSize")
			}
		})
	}
}
