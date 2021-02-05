package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
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
			Capacity: ptr.Int64ToPtr(count),
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

	scaleReq, err := t.generateScaleReq(num, config)
	if err != nil {
		return fmt.Errorf("failed to generate scale in request: %v", err)
	}

	ids, err := t.scaleInUtils.RunPreScaleInTasks(ctx, scaleReq)
	if err != nil {
		return fmt.Errorf("failed to perform Nomad scale in tasks: %v", err)
	}

	// Grab the instanceIDs once as it is used multiple times throughout the
	// scale in event.
	var instanceIDs []string
	for _, node := range ids {

		// RemoteID should be in the format of "{scale-set-name}_{instance-id}"
		// If RemoteID doesn't start vmScaleSet then assume its not part of this scale set.
		// https://docs.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-instance-ids#scale-set-vm-names
		if idx := strings.LastIndex(node.RemoteID, "_"); idx != -1 && strings.EqualFold(node.RemoteID[0:idx], vmScaleSet) {
			instanceIDs = append(instanceIDs, node.RemoteID[idx+1:])
		} else {
			return errors.New("failed to get instance-id from remoteid")
		}
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "resource_group", resourceGroup,
		"vmss_name", vmScaleSet, "instances", instanceIDs)

	// Terminate the detached instances.
	log.Debug("deleting Azure ScaleSet instances")

	future, err := t.vmss.DeleteInstances(ctx, resourceGroup, vmScaleSet, compute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIds: ptr.StringArrToPtr(instanceIDs),
	})

	if err != nil {
		return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
	}

	if err := future.WaitForCompletionRef(ctx, t.vmss.Client); err != nil {
		return fmt.Errorf("failed to scale in Azure ScaleSet: %v", err)
	}

	log.Info("successfully deleted Azure ScaleSet instances")

	// Run any post scale in tasks that are desired.
	if err := t.scaleInUtils.RunPostScaleInTasks(config, ids); err != nil {
		return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
	}

	return nil
}

func (t *TargetPlugin) generateScaleReq(num int64, config map[string]string) (*scaleutils.ScaleInReq, error) {

	// Pull the class key from the config mapping. This is a required value and
	// we cannot scale without this.
	class, ok := config[sdk.TargetConfigKeyClass]
	if !ok {
		return nil, fmt.Errorf("required config param %q not found", sdk.TargetConfigKeyClass)
	}

	// The drain_deadline is an optional parameter so define out default and
	// then attempt to find an operator specified value.
	drain := scaleutils.DefaultDrainDeadline
	ignoreSystemJobs := scaleutils.DefaultIgnoreSystemJobs

	if drainString, ok := config[sdk.TargetConfigKeyDrainDeadline]; ok {
		d, err := time.ParseDuration(drainString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as time duration", drainString)
		}
		drain = d
	}

	if ignoreSystemJobsString, ok := config[sdk.TargetConfigKeyIgnoreSystemJobs]; ok {
		isj, err := strconv.ParseBool(ignoreSystemJobsString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as boolean", ignoreSystemJobsString)
		}
		ignoreSystemJobs = isj
	}

	return &scaleutils.ScaleInReq{
		Num:              int(num),
		DrainDeadline:    drain,
		IgnoreSystemJobs: ignoreSystemJobs,
		PoolIdentifier: &scaleutils.PoolIdentifier{
			IdentifierKey: scaleutils.IdentifierKeyClass,
			Value:         class,
		},
		RemoteProvider: scaleutils.RemoteProviderAzureInstanceID,
		NodeIDStrategy: scaleutils.IDStrategyNewestCreateIndex,
	}, nil
}
