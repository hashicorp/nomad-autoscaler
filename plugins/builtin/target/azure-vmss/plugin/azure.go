// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
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
	var authorizer autorest.Authorizer
	// check for environmental variables, and use if the argument hasn't been set in config
	tenantID := argsOrEnv(config, configKeyTenantID, "ARM_TENANT_ID")
	clientID := argsOrEnv(config, configKeyClientID, "ARM_CLIENT_ID")
	subscriptionID := argsOrEnv(config, configKeySubscriptionID, "ARM_SUBSCRIPTION_ID")
	secretKey := argsOrEnv(config, configKeySecretKey, "ARM_CLIENT_SECRET")

	// Try to use the argument and environment provided arguments first, if this fails fall back to the Azure
	// SDK provided methods
	if tenantID != "" && clientID != "" && secretKey != "" {
		var err error
		authorizer, err = auth.NewClientCredentialsConfig(clientID, secretKey, tenantID).Authorizer()
		if err != nil {
			return fmt.Errorf("azure-vmss (ClientCredentials): %s", err)
		}
	} else {
		var err error
		authorizer, err = auth.NewAuthorizerFromEnvironment()
		if err != nil {
			return fmt.Errorf("azure-vmss (EnvironmentCredentials): %s", err)
		}
	}

	vmss := compute.NewVirtualMachineScaleSetsClient(subscriptionID)
	vmss.Sender = autorest.CreateSender()
	vmss.Authorizer = authorizer

	t.vmss = vmss

	vmssVMs := compute.NewVirtualMachineScaleSetVMsClient(subscriptionID)
	vmssVMs.Sender = autorest.CreateSender()
	vmssVMs.Authorizer = authorizer

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

	future, err := t.vmss.Update(ctx, resourceGroup, vmScaleSet, compute.VirtualMachineScaleSetUpdate{
		Sku: &compute.Sku{
			Capacity: ptr.Of(count),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get the vmss update response: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, t.vmss.Client)
	if err != nil {
		return fmt.Errorf("cannot get the vmss update future response: %v", err)
	}

	log.Info("successfully performed and verified scaling out")
	return nil
}

// scaleIn drain and delete Scale Set instances to match the Autoscaler has deemed required.
func (t *TargetPlugin) scaleIn(ctx context.Context, resourceGroup string, vmScaleSet string, num int64, config map[string]string) error {
	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "resource_group", resourceGroup, "vmss_name", vmScaleSet)

	// Find instance IDs in the target VMSS and perform pre-scale tasks.
	pager, err := t.vmssVMs.List(ctx, resourceGroup, vmScaleSet,
		"startswith(instanceView/statuses/code, 'PowerState') eq true",
		"instanceView/statuses", "instanceView")
	if err != nil {
		return fmt.Errorf("failed to query VMSS instances: %v", err)
	}

	remoteIDs := []string{}
	for pager.NotDone() {
		for _, vm := range pager.Values() {
			for _, s := range *vm.VirtualMachineScaleSetVMProperties.InstanceView.Statuses {
				if strings.HasPrefix(*s.Code, "PowerState/") {
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

		err := pager.NextWithContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to list instances in VMSS: %v", err)
		}
	}

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

	// Terminate the detached instances.
	log.Debug("deleting Azure ScaleSet instances", "instances", instanceIDs)

	future, err := t.vmss.DeleteInstances(ctx, resourceGroup, vmScaleSet, compute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIds: ptr.Of(instanceIDs),
	})

	if err != nil {
		return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
	}

	if err := future.WaitForCompletionRef(ctx, t.vmss.Client); err != nil {
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
