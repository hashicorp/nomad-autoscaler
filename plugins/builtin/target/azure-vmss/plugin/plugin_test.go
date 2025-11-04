// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"strconv"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_calculateDirection(t *testing.T) {
	testCases := []struct {
		inputAsgDesired      int64
		inputStrategyDesired int64
		expectedOutputNum    int64
		expectedOutputString string
		name                 string
	}{
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 11,
			expectedOutputNum:    11,
			expectedOutputString: "out",
			name:                 "scale out desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 9,
			expectedOutputNum:    1,
			expectedOutputString: "in",
			name:                 "scale in desired",
		},
		{
			inputAsgDesired:      10,
			inputStrategyDesired: 10,
			expectedOutputNum:    0,
			expectedOutputString: "",
			name:                 "scale not desired",
		},
	}

	tp := TargetPlugin{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualNum, actualString := tp.calculateDirection(tc.inputAsgDesired, tc.inputStrategyDesired)
			assert.Equal(t, tc.expectedOutputNum, actualNum, tc.name)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
		})
	}
}

func Test_processInstanceView(t *testing.T) {
	testTime := time.Now()

	testCases := []struct {
		inputInstanceView *armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse
		inputStatus       *sdk.TargetStatus
		expectedStatus    *sdk.TargetStatus
		name              string
	}{
		{
			inputInstanceView: nil,
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			name: "nil instance view",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			name: "empty statuses",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{
						{
							Code: stringToPtr("ProvisioningState/creating"),
							Time: &testTime,
						},
					},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: false,
				Count: 1,
				Meta: map[string]string{
					sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(testTime.UnixNano(), 10),
				},
			},
			name: "provisioning in progress",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{
						{
							Code: stringToPtr("ProvisioningState/succeeded"),
							Time: &testTime,
						},
					},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta: map[string]string{
					sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(testTime.UnixNano(), 10),
				},
			},
			name: "provisioning succeeded",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{
						{
							Code: stringToPtr("ProvisioningState/succeeded"),
							Time: nil,
						},
					},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			name: "provisioning succeeded with nil time",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{
						{
							Code: stringToPtr("ProvisioningState/succeeded"),
							Time: &testTime,
						},
						{
							Code: stringToPtr("ProvisioningState/creating"),
							Time: &testTime,
						},
					},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 2,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: false,
				Count: 2,
				Meta: map[string]string{
					sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(testTime.UnixNano(), 10),
				},
			},
			name: "multiple statuses with one not ready",
		},
		{
			inputInstanceView: &armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse{
				VirtualMachineScaleSetInstanceView: armcompute.VirtualMachineScaleSetInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{
						{
							Code: nil,
							Time: &testTime,
						},
					},
				},
			},
			inputStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta:  map[string]string{},
			},
			expectedStatus: &sdk.TargetStatus{
				Ready: true,
				Count: 1,
				Meta: map[string]string{
					sdk.TargetStatusMetaKeyLastEvent: strconv.FormatInt(testTime.UnixNano(), 10),
				},
			},
			name: "nil code",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processInstanceView(tc.inputInstanceView, tc.inputStatus)
			assert.Equal(t, tc.expectedStatus, tc.inputStatus, tc.name)
		})
	}
}

func stringToPtr(v string) *string {
	return &v
}
