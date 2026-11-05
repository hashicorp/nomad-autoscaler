// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/shoenig/test"
)

// ---------------------------------------------------------------------------
// Mock OCI client
// ---------------------------------------------------------------------------

// mockOCIClient implements ociComputeManagementClient for unit tests.
type mockOCIClient struct {
	getInstancePoolFn        func(ctx context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error)
	updateInstancePoolFn     func(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	listInstancePoolInstFn   func(ctx context.Context, req core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error)
	detachInstancePoolInstFn func(ctx context.Context, req core.DetachInstancePoolInstanceRequest) (core.DetachInstancePoolInstanceResponse, error)

	getCallCount    int
	updateCallCount int
	listCallCount   int
	detachCallCount int
	lastDetachReq   core.DetachInstancePoolInstanceRequest
}

func (m *mockOCIClient) GetInstancePool(ctx context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error) {
	m.getCallCount++
	if m.getInstancePoolFn != nil {
		return m.getInstancePoolFn(ctx, req)
	}
	return core.GetInstancePoolResponse{}, nil
}

func (m *mockOCIClient) UpdateInstancePool(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
	m.updateCallCount++
	if m.updateInstancePoolFn != nil {
		return m.updateInstancePoolFn(ctx, req)
	}
	return core.UpdateInstancePoolResponse{}, nil
}

func (m *mockOCIClient) ListInstancePoolInstances(ctx context.Context, req core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
	m.listCallCount++
	if m.listInstancePoolInstFn != nil {
		return m.listInstancePoolInstFn(ctx, req)
	}
	return core.ListInstancePoolInstancesResponse{}, nil
}

func (m *mockOCIClient) DetachInstancePoolInstance(ctx context.Context, req core.DetachInstancePoolInstanceRequest) (core.DetachInstancePoolInstanceResponse, error) {
	m.detachCallCount++
	m.lastDetachReq = req
	if m.detachInstancePoolInstFn != nil {
		return m.detachInstancePoolInstFn(ctx, req)
	}
	return core.DetachInstancePoolInstanceResponse{}, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestPlugin(mock *mockOCIClient) *TargetPlugin {
	return &TargetPlugin{
		config:        map[string]string{},
		logger:        hclog.NewNullLogger(),
		poolClient:    mock,
		retryAttempts: 3,
		retryInterval: time.Microsecond,
	}
}

func makePool(state core.InstancePoolLifecycleStateEnum, currentSize, size int) core.InstancePool {
	now := common.SDKTime{Time: time.Now()}
	return core.InstancePool{
		Id:                      common.String("ocid1.instancepool.test"),
		CompartmentId:           common.String("ocid1.compartment.test"),
		InstanceConfigurationId: common.String("ocid1.instanceconfiguration.test"),
		LifecycleState:          state,
		PlacementConfigurations: []core.InstancePoolPlacementConfiguration{},
		Size:                    common.Int(size),
		CurrentSize:             common.Int(currentSize),
		TimeCreated:             &now,
	}
}

func makeInstance(id, state string) core.InstanceSummary {
	now := common.SDKTime{Time: time.Now()}
	return core.InstanceSummary{
		Id:                      common.String(id),
		AvailabilityDomain:      common.String("AD-1"),
		CompartmentId:           common.String("ocid1.compartment.test"),
		InstanceConfigurationId: common.String("ocid1.instanceconfiguration.test"),
		Region:                  common.String("ap-batam-1"),
		State:                   common.String(state),
		TimeCreated:             &now,
	}
}

// ---------------------------------------------------------------------------
// ociNodeIDMap
// ---------------------------------------------------------------------------

func Test_ociNodeIDMap(t *testing.T) {
	testCases := []struct {
		name                string
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError bool
	}{
		{
			name: "OCID in attributes",
			inputNode: &api.Node{
				ID:         "node-1",
				Attributes: map[string]string{nodeAttrOCIInstanceID: "ocid1.instance.oc1.ap-batam-1.aaa"},
				Meta:       map[string]string{},
			},
			expectedOutputID: "ocid1.instance.oc1.ap-batam-1.aaa",
		},
		{
			name: "OCID in meta (fallback when attributes empty)",
			inputNode: &api.Node{
				ID:         "node-2",
				Attributes: map[string]string{},
				Meta:       map[string]string{nodeAttrOCIInstanceID: "ocid1.instance.oc1.sa-saopaulo-1.bbb"},
			},
			expectedOutputID: "ocid1.instance.oc1.sa-saopaulo-1.bbb",
		},
		{
			name: "attributes take precedence over meta",
			inputNode: &api.Node{
				ID:         "node-3",
				Attributes: map[string]string{nodeAttrOCIInstanceID: "ocid-attr"},
				Meta:       map[string]string{nodeAttrOCIInstanceID: "ocid-meta"},
			},
			expectedOutputID: "ocid-attr",
		},
		{
			name: "OCID not found in attributes or meta",
			inputNode: &api.Node{
				ID:         "node-4",
				Attributes: map[string]string{},
				Meta:       map[string]string{},
			},
			expectedOutputError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := ociNodeIDMap(tc.inputNode)
			if tc.expectedOutputError {
				test.Error(t, actualErr)
				test.StrContains(t, actualErr.Error(), nodeAttrOCIInstanceID)
			} else {
				test.NoError(t, actualErr)
				test.Eq(t, tc.expectedOutputID, actualID)
			}
		})
	}
}
