// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/compute/v1"
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

// mockInstanceGroup implements the instanceGroup interface for the purpose of testing.
type mockInstanceGroup struct {
	name       string
	statusFunc func(context.Context, *compute.Service) (bool, int64, error)
}

func (m *mockInstanceGroup) getName() string {
	return m.name
}

func (m *mockInstanceGroup) status(ctx context.Context, s *compute.Service) (bool, int64, error) {
	return m.statusFunc(ctx, s)
}

func (m *mockInstanceGroup) resize(ctx context.Context, s *compute.Service, num int64) error {
	return nil
}

func (m *mockInstanceGroup) deleteInstance(ctx context.Context, s *compute.Service, instances []string) error {
	return nil
}

func (m *mockInstanceGroup) listInstances(ctx context.Context, s *compute.Service) ([]*compute.ManagedInstance, error) {
	return nil, nil
}

func TestTargetPlugin_ensureInstanceGroupIsStable(t *testing.T) {
	// Test case 1: MIG becomes stable within the retry limit.
	t.Run("mig becomes stable", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 3
		
		attempts := 0
		mockIG := &mockInstanceGroup{
			name: "test-mig-success",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				attempts++
				if attempts <= 2 {
					return false, 0, nil
				}
				return true, 10, nil
			},
		}

		err := tp.ensureInstanceGroupIsStable(context.Background(), mockIG)

		assert.NoError(t, err, "expected no error when MIG becomes stable")
		assert.Equal(t, 3, attempts, "expected 3 attempts to become stable")
	})

	// Test case 2: MIG never becomes stable and reaches the retry limit.
	t.Run("mig never becomes stable", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 2
		
		attempts := 0
		mockIG := &mockInstanceGroup{
			name: "test-mig-failure",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				attempts++
				return false, 0, nil
			},
		}

		err := tp.ensureInstanceGroupIsStable(context.Background(), mockIG)

		assert.Error(t, err, "expected an error when MIG does not become stable")
		assert.Equal(t, 2, attempts, "expected 2 attempts (the limit) to be made")
	})
}
