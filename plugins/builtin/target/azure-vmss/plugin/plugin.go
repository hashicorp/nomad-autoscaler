// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

const (
	// pluginName is the unique name of the this plugin amongst Target plugins.
	pluginName = "azure-vmss"

	// configKeys represents the known configuration parameters required at
	// varying points throughout the plugins lifecycle.
	configKeySubscriptionID = "subscription_id"
	configKeyTenantID       = "tenant_id"
	configKeyClientID       = "client_id"
	configKeySecretKey      = "secret_access_key"
	configKeyResoureGroup   = "resource_group"
	configKeyVMSS           = "vm_scale_set"
)

var (
	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewAzureVMSSPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the Azure VMSS implementation of the target.Target interface.
type TargetPlugin struct {
	config map[string]string
	logger hclog.Logger

	vm      *armcompute.VirtualMachinesClient
	vmss    *armcompute.VirtualMachineScaleSetsClient
	vmssVMs *armcompute.VirtualMachineScaleSetVMsClient

	// clusterUtils provides general cluster scaling utilities for querying the
	// state of nodes pools and performing scaling tasks.
	clusterUtils *scaleutils.ClusterScaleUtils
}

// NewAzureVMSSPlugin returns the Azure VMSS implementation of the target.Target
// interface.
func NewAzureVMSSPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger: log,
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {

	t.logger.Info("setting Azure VMSS target plugin config", "config", config)

	t.config = config

	if err := t.setupAzureClient(config); err != nil {
		return err
	}

	clusterUtils, err := scaleutils.NewClusterScaleUtils(nomad.ConfigFromNamespacedMap(config), t.logger)
	if err != nil {
		return err
	}

	// Store and set the remote ID callback function.
	t.clusterUtils = clusterUtils
	t.clusterUtils.ClusterNodeIDLookupFunc = azureNodeIDMap

	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale satisfies the Scale function on the target.Target interface.
func (t *TargetPlugin) Scale(action sdk.ScalingAction, config map[string]string) error {
	// Azure can't support dry-run like Nomad, so just exit.
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	// We cannot scale an Scale Set without knowing the resource group and name.
	resourceGroup, ok := config[configKeyResoureGroup]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyResoureGroup)
	}
	vmScaleSet, ok := config[configKeyVMSS]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyVMSS)
	}
	ctx := context.Background()

	currVMSS, err := t.vmss.Get(ctx, resourceGroup, vmScaleSet, nil)
	if err != nil {
		return fmt.Errorf("failed to get Azure vmss: %v", err)
	}

	capacity := *currVMSS.SKU.Capacity

	// The Azure VMSS target requires different details depending on which
	// direction we want to scale. Therefore calculate the direction and the
	// relevant number so we can correctly perform the AWS work.
	num, direction := t.calculateDirection(capacity, action.Count)

	orchestrationMode := string(*currVMSS.Properties.OrchestrationMode)

	switch direction {
	case "in":
		err = t.scaleIn(ctx, resourceGroup, vmScaleSet, num, config, orchestrationMode)
	case "out":
		err = t.scaleOut(ctx, resourceGroup, vmScaleSet, num)
	default:
		t.logger.Info("scaling not required", "resource_group", resourceGroup, "vmss", vmScaleSet,
			"current_count", capacity, "strategy_count", action.Count)
		return nil
	}

	// If we received an error while scaling, format this with an outer message
	// so its nice for the operators and then return any error to the caller.
	if err != nil {
		err = fmt.Errorf("failed to perform scaling action: %v", err)
	}
	return err
}

// Status satisfies the Status function on the target.Target interface.
func (t *TargetPlugin) Status(config map[string]string) (*sdk.TargetStatus, error) {

	// Perform our check of the Nomad node pool. If the pool is not ready, we
	// can exit here and avoid calling the Azure API as it won't affect the
	// outcome.
	ready, err := t.clusterUtils.IsPoolReady(config)
	if err != nil {
		return nil, fmt.Errorf("failed to run Nomad node readiness check: %v", err)
	}
	if !ready {
		return &sdk.TargetStatus{Ready: ready}, nil
	}

	// We cannot scale an vmss without knowing the vmss resource group and name.
	resourceGroup, ok := config[configKeyResoureGroup]
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyResoureGroup)
	}
	vmScaleSet, ok := config[configKeyVMSS]
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyVMSS)
	}

	ctx := context.Background()

	t.logger.Debug("getting Azure ScaleSet status", "resource_group", resourceGroup, "vmss_name", vmScaleSet)

	vmss, err := t.vmss.Get(ctx, resourceGroup, vmScaleSet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure ScaleSet: %v", err)
	}

	vmssMode := string(*vmss.Properties.OrchestrationMode)

	// Currently only two orchestration modes are supported - Flexible and Uniform.
	// https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute@v1.0.0#OrchestrationMode
	// Flexible is not compatible with GetInstanceView which is used in Uniform logic.
	// Get all VMs in the VMSS, so we can process them individually later.
	vms := []string{}
	if vmssMode == orchestrationModeFlexible {
		t.logger.Debug("VMSS Orchestration Mode", "mode", vmssMode)

		flexVMs, err := t.getVMSSVMs(ctx, resourceGroup, vmssMode, vmScaleSet)
		if err != nil {
			return nil, fmt.Errorf("failed to get VMSS flexible VMs: %w", err)
		}

		vms = flexVMs

	}

	// GetInstanceView - Gets the status of a VM scale set instance.
	var instanceView armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse
	if vmssMode == orchestrationModeUniform {
		instanceView, err = t.vmss.GetInstanceView(ctx, resourceGroup, vmScaleSet, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get Azure ScaleSet Instance View: %v", err)
		}
	}

	// Set our initial status.
	resp := sdk.TargetStatus{
		Ready: true,
		Count: *vmss.SKU.Capacity,
		Meta:  make(map[string]string),
	}

	// If flexible, it takes the VMSS VMs and processes them individually
	// outside of the scope of the VMSS.
	if vmssMode == orchestrationModeFlexible {
		t.processInstanceViewFlexible(vms, resourceGroup, &resp)
	}

	// If Uniform, we process the instance view directly from the VMSS.
	if vmssMode == orchestrationModeUniform {
		processInstanceView(&instanceView, &resp)
	}

	return &resp, nil
}

func (t *TargetPlugin) calculateDirection(vmssDesired, strategyDesired int64) (int64, string) {

	if strategyDesired < vmssDesired {
		return vmssDesired - strategyDesired, "in"
	}
	if strategyDesired > vmssDesired {
		return strategyDesired, "out"
	}
	return 0, ""
}

// processInstanceView updates the status object based on the details within
// the vmss instances.
func processInstanceView(instanceView *armcompute.VirtualMachineScaleSetsClientGetInstanceViewResponse, status *sdk.TargetStatus) {

	if instanceView == nil || len(instanceView.VirtualMachineScaleSetInstanceView.Statuses) == 0 {
		return
	}

	latestTime := int64(math.MinInt64)
	if instanceView.VirtualMachineScaleSetInstanceView.Statuses != nil {
		for _, instanceStatus := range instanceView.VirtualMachineScaleSetInstanceView.Statuses {
			if instanceStatus.Code != nil && *instanceStatus.Code != "ProvisioningState/succeeded" {
				status.Ready = false
			}

			// Time isn't always populated, especially if the activity has not yet
			// finished :).
			if instanceStatus.Time != nil {
				currentTime := instanceStatus.Time.UnixNano()
				if currentTime > latestTime {
					latestTime = currentTime
					status.Meta[sdk.TargetStatusMetaKeyLastEvent] = strconv.FormatInt(currentTime, 10)
				}
			}
		}
	}
}

func (t *TargetPlugin) processInstanceViewFlexible(vms []string, resourceGroup string, status *sdk.TargetStatus) {
	start := time.Now()

	if len(vms) == 0 {
		t.logger.Debug("No VMs found in the VMSS, skipping instance view processing.")
		return
	}

	t.logger.Debug("Total VMs found in the Flexible VMSS", "count", len(vms))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	requests := make(chan struct{}, 5)

	var notReady atomic.Bool
	var statusReady = true

	for _, vmName := range vms {
		if ctx.Err() != nil {
			t.logger.Debug("Context cancelled, stopping further processing of VMs")
			break
		}

		wg.Add(1)
		requests <- struct{}{}

		go func(vm string) {
			defer wg.Done()
			defer func() { <-requests }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			instanceView, err := t.vm.InstanceView(ctx, resourceGroup, vm, nil)
			if err != nil {
				t.logger.Debug("Failed to get instance view for VM", "vm_name", vm, "error", err)
				return
			}

			if !isFlexibleVMReady(instanceView.Statuses) {
				t.logger.Debug("VM not ready", "vm_name", vm, "statuses", instanceView.Statuses)

				if notReady.CompareAndSwap(false, true) {
					t.logger.Debug("Setting status to not ready and cancelling context", "vm_name", vm)
					statusReady = false
					cancel()
				}
				return
			}

			t.logger.Debug("VM instance view is ready", "vm_name", vm)
		}(vmName)
	}
	wg.Wait()

	t.logger.Debug("Setting status", statusReady)
	status.Ready = statusReady
	t.logger.Debug("Finished processing VM instance views", "duration_seconds", time.Since(start).Seconds())
}
