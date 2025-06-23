// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
)

const (

	// The two orchestration modes supported by Azure VMSS.
	// Keeping naming consistent with the Azure SDK for Go.
	// https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute@v1.0.0#OrchestrationMode
	orchestrationModeFlexible = "Flexible"
	orchestrationModeUniform  = "Uniform"

	nodeAttrAzureInstanceID = "unique.platform.azure.name"
)

// argsOrEnv allows you to pick an environmental variable for a setting if the arg is not set
func argsOrEnv(args map[string]string, key, env string) string {
	if value, ok := args[key]; ok {
		return value
	}
	return os.Getenv(env)
}

// setupAzureClients takes the passed config mapping and instantiates the
// required Azure service clients.
func (t *TargetPlugin) setupAzureClient(config map[string]string) error {
	tenantID := argsOrEnv(config, configKeyTenantID, "ARM_TENANT_ID")
	clientID := argsOrEnv(config, configKeyClientID, "ARM_CLIENT_ID")
	subscriptionID := argsOrEnv(config, configKeySubscriptionID, "ARM_SUBSCRIPTION_ID")
	secretKey := argsOrEnv(config, configKeySecretKey, "ARM_CLIENT_SECRET")

	if tenantID == "" || clientID == "" || subscriptionID == "" || secretKey == "" {
		return fmt.Errorf("missing required Azure configuration: tenant_id, client_id, subscription_id, and secret_access_key are required")
	}

	// Create a new Azure client secret credential using the provided configuration.
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, secretKey, nil)
	if err != nil {
		return fmt.Errorf("failed to create Azure client secret credential: %w", err)
	}

	vm, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}
	t.vm = vm

	vmss, err := armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS client: %w", err)
	}
	t.vmss = vmss

	vmssVMs, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS VMs client: %w", err)
	}
	t.vmssVMs = vmssVMs

	return nil
}

// scaleOut updates the Scale Set desired count to match what the
// Autoscaler has deemed required.
func (t *TargetPlugin) scaleOut(ctx context.Context, resourceGroup string, vmScaleSet string, count int64) error {

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_out", "vmss_name", vmScaleSet,
		"desired_count", count)

	future, err := t.vmss.BeginUpdate(ctx, resourceGroup, vmScaleSet, armcompute.VirtualMachineScaleSetUpdate{
		SKU: &armcompute.SKU{
			Capacity: ptr.Of(count),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to get the vmss update response: %v", err)
	}

	_, err = future.Poll(ctx)
	if err != nil {
		return fmt.Errorf("cannot get the vmss update future response: %v", err)
	}

	log.Info("successfully performed and verified scaling out")
	return nil
}

// scaleIn drain and delete Scale Set instances to match the Autoscaler has deemed required.
func (t *TargetPlugin) scaleIn(ctx context.Context, resourceGroup string, vmScaleSet string, num int64, config map[string]string, vmssMode string) error {
	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "resource_group", resourceGroup, "vmss_name", vmScaleSet)

	// Get all VMs in VMSS
	log.Debug("getting VMs in VMSS")
	vms, err := t.getVMSSVMs(ctx, resourceGroup, vmssMode, vmScaleSet)
	if err != nil {
		return fmt.Errorf("failed to get VMs in VMSS: %w", err)
	}

	// early exit if no VMs found,
	// should not happen, but checking.
	if len(vms) == 0 {
		return fmt.Errorf("no VMs found in VMSS")
	}

	// early exit if not enough total VMs found between status check and scale in actions,
	// should not happen, but checking.
	if num > int64(len(vms)) {
		return fmt.Errorf("cannot scale in %d instances, only %d total", num, len(vms))
	}

	// additional processing needed for flexible mode to get status of VMs
	// for uniform, populate remoteIDs with VMs
	var remoteIDs []string
	log.Debug("getting ready remote IDs for Azure ScaleSet instances")
	switch vmssMode {
	case orchestrationModeUniform:
		remoteIDs = vms

		// Should not get here, but adding.
		if len(remoteIDs) == 0 {
			return fmt.Errorf("no ")
		}

	case orchestrationModeFlexible:
		remoteIDs, err = t.getFlexibleReadyRemoteIDs(ctx, resourceGroup, vms)
		if err != nil {
			return fmt.Errorf("failed to get remote IDs: %w", err)
		}
	default:
		return fmt.Errorf("unsupported VMSS mode: %s", vmssMode)
	}

	// run pre-scale tasks using remoteIDs
	log.Debug("starting pre-scale tasks for Azure ScaleSet instances", "remote_ids", remoteIDs)
	ids, err := t.clusterUtils.RunPreScaleInTasksWithRemoteCheck(ctx, config, remoteIDs, int(num))
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
	}

	if len(ids) == 0 {
		return fmt.Errorf("no ids generated")
	}

	// Grab the instanceIDs once as it is used multiple times throughout the
	// scale in event.
	var instanceIDs []string
	switch vmssMode {
	case orchestrationModeUniform:
		for _, node := range ids {
			log.Debug("processing node for scale in", "node_id", node, "remote_id", node.RemoteResourceID)
			// RemoteID should be in the format of "{scale-set-name}_{instance-id}"
			// If RemoteID doesn't start vmScaleSet then assume its not part of this scale set.
			// https://docs.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-instance-ids#scale-set-vm-names
			if idx := strings.LastIndex(node.RemoteResourceID, "_"); idx != -1 && strings.EqualFold(node.RemoteResourceID[0:idx], vmScaleSet) {
				instanceIDs = append(instanceIDs, node.RemoteResourceID[idx+1:])
			} else {
				return errors.New("failed to get instance-id from remoteid")
			}
		}
	case orchestrationModeFlexible:
		for _, node := range ids {
			log.Debug("processing node for scale in", "node_id", node, "remote_id", node.RemoteResourceID)
			// You'll notice that the logic here is different than Uniform mode. This is mainly due to Azure's consistency of being inconsistent.
			// https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-instance-ids#scale-set-vm-names
			if idx := strings.LastIndex(node.RemoteResourceID, "_"); idx != -1 &&
				strings.EqualFold(node.RemoteResourceID[0:idx], vmScaleSet) {
				instanceIDs = append(instanceIDs, node.RemoteResourceID)

			} else {
				return errors.New("failed to validate remoteid format")
			}
		}
	default:
		// if the orchestration mode is not supported or validated
		// should not get to this point
		return errors.New("unsupported orchestration mode")
	}

	if len(instanceIDs) == 0 {
		return errors.New("no instancesIDs generated")
	}

	// convert the instanceIDs to a slice of pointers for the Azure SDK.
	instanceIDPtrs := make([]*string, 0, len(instanceIDs))

	for _, id := range instanceIDs {
		instanceIDPtrs = append(instanceIDPtrs, &id)
	}

	// Terminate the detached instances.
	log.Debug("deleting Azure ScaleSet instances", "instances", instanceIDs)

	future, err := t.vmss.BeginDeleteInstances(ctx, resourceGroup, vmScaleSet, armcompute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIDs: instanceIDPtrs,
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to scale in Azure ScaleSet: %w", err)
	}

	_, err = future.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
	}

	log.Info("successfully deleted Azure ScaleSet instances")

	// Run any post scale in tasks that are desired.
	if err := t.clusterUtils.RunPostScaleInTasks(ctx, config, ids); err != nil {
		return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
	}

	return nil
}

// azureNodeIDMap is used to identify the Azure InstanceID of a Nomad node using
// the relevant attribute value.
func azureNodeIDMap(n *api.Node) (string, error) {
	if val, ok := n.Attributes[nodeAttrAzureInstanceID]; ok {
		return val, nil
	}

	// Fallback to meta tag.
	if val, ok := n.Meta[nodeAttrAzureInstanceID]; ok {
		return val, nil
	}

	return "", fmt.Errorf("attribute %q not found", nodeAttrAzureInstanceID)
}

// getVMSSVMs to get VM names.
// handles both uniform and flexible VMSS modes.
func (t *TargetPlugin) getVMSSVMs(ctx context.Context, resourceGroup string, vmssMode string, vmScaleSet string) (vms []string, err error) {
	var vmNames []string

	options := &armcompute.VirtualMachineScaleSetVMsClientListOptions{}

	// If the VMSS mode is uniform, expand the instance view to get the instance ID.
	if vmssMode == orchestrationModeUniform {
		options.Expand = ptr.Of("instanceView")
	}

	pager := t.vmssVMs.NewListPager(resourceGroup, vmScaleSet, options)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list VMSS instances: %w", err)
		}

		for _, vm := range page.Value {
			if vm.Name == nil || vm.InstanceID == nil {
				continue
			}

			// Check for PowerState/running if instanceView is available.
			// Unable to get flexible instanceView from the VMSS instance.
			if vm.Properties != nil && vm.Properties.InstanceView != nil && vm.Properties.InstanceView.Statuses != nil {
				for _, s := range vm.Properties.InstanceView.Statuses {
					if s.Code != nil && strings.HasPrefix(*s.Code, "PowerState/") {
						if *s.Code == "PowerState/running" {
							t.logger.Debug("found healthy instance", "name", *vm.Name, "instance_id", *vm.InstanceID)
							vmNames = append(vmNames, *vm.Name)
							break
						} else {
							t.logger.Debug("skipping instance - power state is not running", "name", *vm.Name, "instance_id", *vm.InstanceID, "code", *s.Code)
						}
					}
				}
			} else if vmssMode == orchestrationModeFlexible {
				// If mode is flexible, we cannot get the instanceView.
				t.logger.Debug("adding instance by default for flexible mode", "name", *vm.Name, "instance_id", *vm.InstanceID)
				vmNames = append(vmNames, *vm.Name)
			} else {
				// Defaults to previous logic with uniform scale sets.
				t.logger.Debug("skipping instance", "id", *vm.ID, "instance_id", *vm.InstanceID)
			}

		}
	}

	return vmNames, nil
}

func (t TargetPlugin) getFlexibleReadyRemoteIDs(ctx context.Context, resourceGroup string, vms []string) (ids []string, err error) {

	remoteIDs := []string{}

	var wg sync.WaitGroup
	// Manually limiting the number of concurrent requests to avoid hitting API rate limits.
	requests := make(chan struct{}, 5)
	mu := sync.Mutex{}

	for _, vm := range vms {
		wg.Add(1)
		requests <- struct{}{}

		go func(vm string) {
			defer wg.Done()
			defer func() { <-requests }()

			select {
			case <-ctx.Done():
				return
			default:
				// Continue processing
			}

			// Get the instance view for the VM in the Flexible VMSS.
			instanceView, err := t.vm.InstanceView(ctx, resourceGroup, vm, nil)
			if err != nil {
				t.logger.Debug("failed to get instance view for VM", "vm_name", vm, "error", err)
				return
			}

			// Checks if instance is running and successfully provisioned.
			var provisioningReady, powerRunning bool
			for _, s := range instanceView.Statuses {
				if s.Code == nil {
					continue
				}

				switch *s.Code {
				case "ProvisioningState/succeeded":
					provisioningReady = true
				case "PowerState/running":
					powerRunning = true
				default:
					t.logger.Debug("unexpected status code", "vm_name", vm, "status_code", *s.Code)
				}
			}

			if !provisioningReady || !powerRunning {
				t.logger.Debug("instance not ready", "vm_name", vm, "provisioning_ready", provisioningReady, "power_ready", powerRunning)
				return
			}

			t.logger.Debug("instance candidate for scale in action", "vm_name", vm)

			mu.Lock()
			remoteIDs = append(remoteIDs, vm)
			mu.Unlock()

		}(vm)
	}
	wg.Wait()

	if len(remoteIDs) == 0 {
		t.logger.Error("no instances found to be provisioned and running.")
		return nil, err
	}

	return remoteIDs, nil
}
