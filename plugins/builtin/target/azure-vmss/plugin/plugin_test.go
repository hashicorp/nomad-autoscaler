package plugin

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest/date"
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

	testTime := time.Date(2020, time.April, 13, 8, 4, 0, 0, time.UTC)

	testCases := []struct {
		inputInstanceView compute.VirtualMachineScaleSetInstanceView
		inputStatus       *sdk.TargetStatus
		expectedStatus    *sdk.TargetStatus
		name              string
	}{
		{
			inputInstanceView: compute.VirtualMachineScaleSetInstanceView{
				VirtualMachine: &compute.VirtualMachineScaleSetInstanceViewStatusesSummary{
					StatusesSummary: &[]compute.VirtualMachineStatusCodeCount{
						{
							Code:  stringToPtr("ProvisioningState/creating"),
							Count: int32ToPtr(1),
						},
					},
				},
				Statuses: &[]compute.InstanceViewStatus{
					{
						Code: stringToPtr("ProvisioningState/creating"),
						Time: nil,
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
				Meta:  map[string]string{},
			},
			name: "InstanceView still in progress",
		},
		{
			inputInstanceView: compute.VirtualMachineScaleSetInstanceView{
				VirtualMachine: &compute.VirtualMachineScaleSetInstanceViewStatusesSummary{
					StatusesSummary: &[]compute.VirtualMachineStatusCodeCount{
						{
							Code:  stringToPtr("ProvisioningState/succeeded"),
							Count: int32ToPtr(1),
						},
						{
							Code:  stringToPtr("ProvisioningState/creating"),
							Count: int32ToPtr(1),
						},
					},
				},
				Statuses: &[]compute.InstanceViewStatus{
					{
						Code: stringToPtr("ProvisioningState/succeeded"),
						Time: nil,
					},
					{
						Code: stringToPtr("ProvisioningState/creating"),
						Time: nil,
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
				Meta:  map[string]string{},
			},
			name: "InstanceView still in progress",
		},
		{
			inputInstanceView: compute.VirtualMachineScaleSetInstanceView{
				VirtualMachine: &compute.VirtualMachineScaleSetInstanceViewStatusesSummary{
					StatusesSummary: &[]compute.VirtualMachineStatusCodeCount{
						{
							Code:  stringToPtr("ProvisioningState/succeeded"),
							Count: int32ToPtr(1),
						},
					},
				},
				Statuses: &[]compute.InstanceViewStatus{
					{
						Code: stringToPtr("ProvisioningState/succeeded"),
						Time: &date.Time{Time: testTime},
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
					"nomad_autoscaler.last_event": "1586765040000000000",
				},
			},
			name: "InstanceView with not nil time",
		},
		{
			inputInstanceView: compute.VirtualMachineScaleSetInstanceView{
				VirtualMachine: &compute.VirtualMachineScaleSetInstanceViewStatusesSummary{
					StatusesSummary: &[]compute.VirtualMachineStatusCodeCount{
						{
							Code:  stringToPtr("ProvisioningState/succeeded"),
							Count: int32ToPtr(1),
						},
					},
				},
				Statuses: &[]compute.InstanceViewStatus{
					{
						Code: stringToPtr("ProvisioningState/succeeded"),
						Time: nil,
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
			name: "InstanceView with nil time",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processInstanceView(tc.inputInstanceView, tc.inputStatus)
			assert.Equal(t, tc.expectedStatus, tc.inputStatus, tc.name)
		})
	}
}

func int32ToPtr(v int32) *int32 {
	return &v
}

func stringToPtr(v string) *string {
	return &v
}
