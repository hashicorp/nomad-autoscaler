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

const nodeAttrAzureInstanceID = "unique.platform.azure.name"

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

	// Needed for flexible VMSS
	// As of 4 June 2024, VMSS Get Instance View is not supported for flexible orchestration mode.
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
func (t *TargetPlugin) scaleOut(ctx context.Context, resourceGroup string, vmScaleSet string, count int64, vmssMode string) error {

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

	resp, err := future.Poll(ctx)
	if err != nil {
		return fmt.Errorf("cannot get the vmss update future response: %v", err)
	}

	t.logger.Debug("vmss update response", "response", resp.Body)

	log.Info("successfully performed and verified scaling out")
	return nil
}

// scaleIn drain and delete Scale Set instances to match the Autoscaler has deemed required.
func (t *TargetPlugin) scaleIn(ctx context.Context, resourceGroup string, vmScaleSet string, num int64, config map[string]string, vmssMode string) error {
	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "resource_group", resourceGroup, "vmss_name", vmScaleSet)

	// Find instance IDs in the target VMSS and perform pre-scale tasks.
	// pager, err := t.vmssVMs.List(ctx, resourceGroup, vmScaleSet,
	// 	"startswith(instanceView/statuses/code, 'PowerState') eq true",
	// 	"instanceView/statuses", "instanceView")
	// if err != nil {
	// 	return fmt.Errorf("failed to query VMSS instances: %v", err)
	// }

	log.Debug("listing Azure ScaleSet instances", "vmss_name", vmScaleSet)

	if vmssMode == "Uniform" {

		// Find instance IDs in the target VMSS and perform pre-scale tasks.
		pager := t.vmssVMs.NewListPager(resourceGroup, vmScaleSet, &armcompute.VirtualMachineScaleSetVMsClientListOptions{
			Expand: ptr.Of("instanceView"),
			// Power State filter is not supported in the Azure SDK for Go, so we
			// filter the results manually later.
			// Filter: ptr.Of("startswith(instanceView/statuses/code, 'PowerState') eq true"),
		})

		log.Debug("iterating over Azure ScaleSet instances", "vmss_name", vmScaleSet)

		remoteIDs := []string{}
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list instances in VMSS: %v", err)
			}
			for _, vm := range page.Value {
				if vm.Properties != nil && vm.Properties.InstanceView != nil && vm.Properties.InstanceView.Statuses != nil {
					for _, s := range vm.Properties.InstanceView.Statuses {
						if s.Code != nil && strings.HasPrefix(*s.Code, "PowerState/") {
							if *s.Code == "PowerState/running" {
								log.Debug("found healthy instance", "id", *vm.ID, "instance_id", *vm.InstanceID)
								remoteIDs = append(remoteIDs, fmt.Sprintf("%s_%s", vmScaleSet, *vm.InstanceID))
							} else {
								log.Debug("skipping instance", "id", *vm.ID, "instance_id", *vm.InstanceID, "code", *s.Code)
							}
							break
						}
					}
				}
			}
		}

		log.Debug("starting pre-scale tasks for Azure ScaleSet instances", "remote_ids", remoteIDs)
		ids, err := t.clusterUtils.RunPreScaleInTasksWithRemoteCheck(ctx, config, remoteIDs, int(num))
		if err != nil {
			return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
		}

		// Grab the instanceIDs once as it is used multiple times throughout the
		// scale in event.
		var instanceIDs []string
		for _, node := range ids {

			// RemoteID should be in the format of "{scale-set-name}_{instance-id}"
			// If RemoteID doesn't start vmScaleSet then assume its not part of this scale set.
			// https://docs.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-instance-ids#scale-set-vm-names
			if idx := strings.LastIndex(node.RemoteResourceID, "_"); idx != -1 && strings.EqualFold(node.RemoteResourceID[0:idx], vmScaleSet) {
				instanceIDs = append(instanceIDs, node.RemoteResourceID[idx+1:])
			} else {
				return errors.New("failed to get instance-id from remoteid")
			}
		}

		// Not sure if there is a better way to do this, but we need to convert
		// the instanceIDs to a slice of pointers for the Azure SDK.
		instanceIDPtrs := make([]*string, 0, len(instanceIDs))

		for _, id := range instanceIDs {
			idCopy := id // ensure a new variable for correct reference
			instanceIDPtrs = append(instanceIDPtrs, &idCopy)
		}

		// Terminate the detached instances.
		log.Debug("deleting Azure ScaleSet instances", "instances", instanceIDs)

		future, err := t.vmss.BeginDeleteInstances(ctx, resourceGroup, vmScaleSet, armcompute.VirtualMachineScaleSetVMInstanceRequiredIDs{
			InstanceIDs: instanceIDPtrs,
		}, nil)

		if err != nil {
			return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
		}

		var resp armcompute.VirtualMachineScaleSetsClientDeleteInstancesResponse
		resp, err = future.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
		}

		log.Debug("Scale in response", "response", resp)

		log.Info("successfully deleted Azure ScaleSet instances")

		// Run any post scale in tasks that are desired.
		if err := t.clusterUtils.RunPostScaleInTasks(ctx, config, ids); err != nil {
			return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
		}
	}

	if vmssMode == "Flexible" {

		log.Debug("Scaling in Azure VMSS with Flexible Orchestration Mode")

		// Find instance IDs in the target VMSS and perform pre-scale tasks.

		vms := &[]armcompute.VirtualMachineScaleSetVM{}
		err, flexVMs := t.getVMSSFlexibleVMs(ctx, resourceGroup, vmScaleSet)
		if err != nil {
			return fmt.Errorf("failed to get VMSS flexible VMs: %v", err)
		}

		vms = &flexVMs

		log.Debug("iterating over Azure ScaleSet instances", "vmss_name", vmScaleSet)

		remoteIDs := []string{}
		var wg sync.WaitGroup
		// Manually limiting the number of concurrent requests to avoid hitting API rate limits.
		requests := make(chan struct{}, 5)
		mu := sync.Mutex{}

		for _, vm := range *vms {
			wg.Add(1)
			requests <- struct{}{}

			go func(vm armcompute.VirtualMachineScaleSetVM) {
				defer wg.Done()
				defer func() { <-requests }()

				select {
				case <-ctx.Done():
					return
				default:
				}

				instanceView, err := t.vm.InstanceView(ctx, resourceGroup, *vm.Name, nil)
				if err != nil {
					t.logger.Debug("failed to get instance view for VM", "vm_name", *vm.InstanceID, "error", err)
					return
				}

				if len(instanceView.Statuses) == 0 || *instanceView.Statuses[0].Code != "ProvisioningState/succeeded" {
					t.logger.Debug("VM instance view not ready", "vm_name", *vm.InstanceID, "status_code", *instanceView.Statuses[0].Code)
					return
				}

				log.Debug("instance view is ready", "vm_name", *vm.InstanceID, "status_code", *instanceView.Statuses[0].Code)

				mu.Lock()
				remoteIDs = append(remoteIDs, *vm.InstanceID)
				mu.Unlock()

			}(vm)
		}
		wg.Wait()

		log.Debug("starting pre-scale tasks for Azure ScaleSet instances", "remote_ids", remoteIDs)
		ids, err := t.clusterUtils.RunPreScaleInTasksWithRemoteCheck(ctx, config, remoteIDs, int(num))
		if err != nil {
			return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
		}

		log.Debug("pre-scale tasks completed", "ids", ids)

		if len(ids) == 0 {
			log.Info("no instances to scale in, exiting")
			return nil
		}

		log.Debug("found instances to scale in", "count", len(ids))

		// Grab the instanceIDs once as it is used multiple times throughout the
		// scale in event.
		var instanceIDs []string
		for _, node := range ids {

			log.Debug("processing node for scale in", "node_id", node, "remote_id", node.RemoteResourceID)

			if idx := strings.LastIndex(node.RemoteResourceID, "_"); idx != -1 &&
				strings.EqualFold(node.RemoteResourceID[0:idx], vmScaleSet) {

				// Keep the full RemoteResourceID instead of slicing off the instance ID
				instanceIDs = append(instanceIDs, node.RemoteResourceID)

			} else {
				return errors.New("failed to validate remoteid format")
			}
		}

		log.Debug("found instance IDs", "ids", instanceIDs)

		// Not sure if there is a better way to do this, but we need to convert
		// the instanceIDs to a slice of pointers for the Azure SDK.
		instanceIDPtrs := make([]*string, 0, len(instanceIDs))

		for _, id := range instanceIDs {
			idCopy := id // ensure a new variable for correct reference
			instanceIDPtrs = append(instanceIDPtrs, &idCopy)
		}

		// Terminate the detached instances.
		log.Debug("deleting Azure ScaleSet instances", "instances", instanceIDs)

		future, err := t.vmss.BeginDeleteInstances(ctx, resourceGroup, vmScaleSet, armcompute.VirtualMachineScaleSetVMInstanceRequiredIDs{
			InstanceIDs: instanceIDPtrs,
		}, nil)

		if err != nil {
			return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
		}

		var resp armcompute.VirtualMachineScaleSetsClientDeleteInstancesResponse
		resp, err = future.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
		}

		log.Debug("Scale in response", "response", resp)

		log.Info("successfully deleted Azure ScaleSet instances")

		// Run any post scale in tasks that are desired.
		if err := t.clusterUtils.RunPostScaleInTasks(ctx, config, ids); err != nil {
			return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
		}
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

// Ended up making this a method to simplify.
func (t *TargetPlugin) getVMSSFlexibleVMs(ctx context.Context, resourceGroup string, vmScaleSet string) (err error, vms []armcompute.VirtualMachineScaleSetVM) {

	pager := t.vmssVMs.NewListPager(resourceGroup, vmScaleSet, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list VMSS instances: %v", err), nil
		}

		for _, vm := range page.Value {
			vms = append(vms, *vm)
		}
	}
	return nil, vms
}
