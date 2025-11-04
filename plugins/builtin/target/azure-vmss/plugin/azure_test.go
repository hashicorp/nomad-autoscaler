// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_azureNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError error
		name                string
	}{
		{
			inputNode: &api.Node{
				Attributes: map[string]string{"unique.platform.azure.name": "13f56399-bd52-4150-9748-7190aae1ff21"},
			},
			expectedOutputID:    "13f56399-bd52-4150-9748-7190aae1ff21",
			expectedOutputError: nil,
			name:                "required attribute found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
				Meta:       map[string]string{"unique.platform.azure.name": "13f56399-bd52-4150-9748-7190aae1ff21"},
			},
			expectedOutputID:    "13f56399-bd52-4150-9748-7190aae1ff21",
			expectedOutputError: nil,
			name:                "required fallback meta found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.platform.azure.name" not found`),
			name:                "required attribute not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := azureNodeIDMap(tc.inputNode)
			assert.Equal(t, tc.expectedOutputID, actualID, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualErr, tc.name)
		})
	}
}

func Test_isFlexibleVMReady(t *testing.T) {
	testCases := []struct {
		name     string
		statuses []*armcompute.InstanceViewStatus
		expected bool
	}{
		{
			name: "both provisioned and running present",
			statuses: func() []*armcompute.InstanceViewStatus {
				a := "ProvisioningState/succeeded"
				b := "PowerState/running"
				return []*armcompute.InstanceViewStatus{
					{Code: &a},
					{Code: &b},
				}
			}(),
			expected: true,
		},
		{
			name:     "empty",
			statuses: []*armcompute.InstanceViewStatus{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isFlexibleVMReady(tc.statuses)
			assert.Equal(t, tc.expected, actual, tc.name)
		})
	}
}

// Flexible orchestration mode instance name: {scale-set-name}_{8-char-guid}
// Uniform orchestration mode instance name: {scale-set-name}_{instance-id}
func Test_idFromRemoteID(t *testing.T) {
	testCases := []struct {
		name           string
		mode           string
		remoteResource string
		vmScaleSet     string
		expectedID     string
		expectError    bool
	}{
		{
			name:           "uniform valid",
			mode:           orchestrationModeUniform,
			remoteResource: "myScaleSet_42",
			vmScaleSet:     "myScaleSet",
			expectedID:     "42",
			expectError:    false,
		},
		{
			name:           "flexible valid",
			mode:           orchestrationModeFlexible,
			remoteResource: "myFlexibleVM_abc123",
			vmScaleSet:     "myFlexibleVM",
			expectedID:     "myFlexibleVM_abc123",
			expectError:    false,
		},
		{
			name:           "uniform invalid name format",
			mode:           orchestrationModeUniform,
			remoteResource: "myScaleSet42",
			vmScaleSet:     "myScaleSet",
			expectedID:     "",
			expectError:    true,
		},
		{
			name:           "flexible invalid name format",
			mode:           orchestrationModeFlexible,
			remoteResource: "myFlexibleVMabc123",
			vmScaleSet:     "incorrectscaleset",
			expectedID:     "",
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := idFromRemoteID(tc.mode, tc.remoteResource, tc.vmScaleSet)
			if tc.expectError {
				assert.Error(t, err, tc.name)
			} else {
				assert.NoError(t, err, tc.name)
				assert.Equal(t, tc.expectedID, got, tc.name)
			}
		})
	}
}
