// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/core"
)

const (
	// nodeAttrOCIInstanceID is the Nomad node attribute that holds the OCI
	// instance OCID. The Nomad client on each OCI VM must set this so the
	// plugin can map a Nomad node → OCI instance OCID for scale-in.
	//
	//   client {
	//     meta {
	//       "unique.platform.oci.instance_id" = "<instance-ocid>"
	//     }
	//   }
	nodeAttrOCIInstanceID = "unique.platform.oci.instance_id"

	lifecycleStateRunning = "RUNNING"

	// configKeyUseInstancePrincipal enables instance-principal auth mode.
	// Set "true" when the autoscaler runs on an OCI VM in a dynamic group
	// that has ComputeManagement write policy.
	configKeyUseInstancePrincipal = "use_instance_principal"

	// configKeyOCIConfigFile is the path to an OCI SDK config file.
	// When set, credentials are read from this file instead of ~/.oci/config.
	configKeyOCIConfigFile = "oci_config_file"

	// configKeyOCIConfigProfile is the profile within the OCI config file.
	// Defaults to "DEFAULT".
	configKeyOCIConfigProfile = "oci_config_profile"
)

// ociComputeManagementClient is the subset of core.ComputeManagementClient
// methods used by this plugin. Defining it as an interface allows tests to
// inject a mock without requiring a live OCI endpoint.
type ociComputeManagementClient interface {
	GetInstancePool(ctx context.Context, request core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error)
	UpdateInstancePool(ctx context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	ListInstancePoolInstances(ctx context.Context, request core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error)
	DetachInstancePoolInstance(ctx context.Context, request core.DetachInstancePoolInstanceRequest) (core.DetachInstancePoolInstanceResponse, error)
}

var _ ociComputeManagementClient = core.ComputeManagementClient{}

// setupOCIClient initialises the OCI ComputeManagement client. Three
// credential modes are supported (evaluated in order):
//
//  1. Instance-principal — set `use_instance_principal = "true"`.
//  2. Explicit config file — set `oci_config_file = "/path"`.
//  3. Default credential chain — falls back to ~/.oci/config, profile DEFAULT.
func (t *TargetPlugin) setupOCIClient(config map[string]string) error {
	provider, err := buildConfigProvider(config)
	if err != nil {
		return fmt.Errorf("failed to build OCI config provider: %w", err)
	}

	client, err := core.NewComputeManagementClientWithConfigurationProvider(provider)
	if err != nil {
		return fmt.Errorf("failed to create OCI ComputeManagement client: %w", err)
	}

	// Override the region when the autoscaler runs outside OCI (e.g. on AWS).
	if region, ok := t.getValue(config, configKeyRegion); ok && region != "" {
		client.SetRegion(region)
	}

	t.poolClient = client
	return nil
}

func buildConfigProvider(config map[string]string) (common.ConfigurationProvider, error) {
	if v, ok := config[configKeyUseInstancePrincipal]; ok && v == "true" {
		provider, err := auth.InstancePrincipalConfigurationProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to create OCI instance principal provider: %w", err)
		}
		return provider, nil
	}

	if filePath, ok := config[configKeyOCIConfigFile]; ok {
		profile := getConfigValue(config, configKeyOCIConfigProfile, "DEFAULT")
		return common.CustomProfileConfigProvider(filePath, profile), nil
	}

	return common.DefaultConfigProvider(), nil
}

// poolStatus queries the OCI Instance Pool and returns:
//   - current instance count (CurrentSize; falls back to Size when nil)
//   - whether the pool is ready (LifecycleState == "RUNNING")
func (t *TargetPlugin) poolStatus(ctx context.Context, poolID string) (int64, bool, error) {
	resp, err := t.poolClient.GetInstancePool(ctx, core.GetInstancePoolRequest{
		InstancePoolId: common.String(poolID),
	})
	if err != nil {
		return 0, false, fmt.Errorf("GetInstancePool(%s): %w", poolID, err)
	}

	pool := resp.InstancePool
	state := string(pool.LifecycleState)

	var count int64
	if pool.CurrentSize != nil {
		count = int64(*pool.CurrentSize)
	} else if pool.Size != nil {
		count = int64(*pool.Size)
	}

	ready := state == lifecycleStateRunning

	t.logger.Debug("OCI instance pool status",
		"pool_id", poolID,
		"lifecycle_state", state,
		"count", count,
		"ready", ready,
	)

	return count, ready, nil
}

func (t *TargetPlugin) scaleOut(ctx context.Context, poolID string, count int64, config map[string]string) error {
	log := t.logger.With("action", "scale_out", "pool_id", poolID, "desired_count", count)

	_, err := t.poolClient.UpdateInstancePool(ctx, core.UpdateInstancePoolRequest{
		InstancePoolId: common.String(poolID),
		UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
			Size: common.Int(int(count)),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update OCI instance pool size: %w", err)
	}

	retryAttempts, err := t.resolveRetryAttempts(config)
	if err != nil {
		return err
	}

	if err := t.ensurePoolConverged(ctx, poolID, count, retryAttempts); err != nil {
		return fmt.Errorf("failed to confirm scale-out: %w", err)
	}
	log.Info("successfully scaled out OCI instance pool")
	return nil
}

// scaleIn drains and purges selected Nomad nodes, then detaches the corresponding OCI instances.
func (t *TargetPlugin) scaleIn(ctx context.Context, poolID string, num int64, config map[string]string) error {
	log := t.logger.With("action", "scale_in", "pool_id", poolID, "count", num)

	// Record the current count before any changes so we can compute the
	// target count for convergence polling.
	currentCount, _, err := t.poolStatus(ctx, poolID)
	if err != nil {
		return fmt.Errorf("failed to get OCI instance pool status before scale-in: %w", err)
	}

	instanceOCIDs, err := t.listPoolInstances(ctx, poolID, config)
	if err != nil {
		return fmt.Errorf("failed to list OCI instance pool members: %w", err)
	}

	if int64(len(instanceOCIDs)) < num {
		// Instances are transitioning (provisioning or terminating) — skip this cycle.
		log.Warn("fewer running instances than requested scale-in count; pool may be stabilising",
			"requested_scale_in", num,
			"running_instances", len(instanceOCIDs),
			"pool_reported_size", currentCount,
		)
		return nil
	}

	ids, err := t.clusterUtils.RunPreScaleInTasksWithRemoteCheck(ctx, config, instanceOCIDs, int(num))
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale-in tasks: %w", err)
	}

	if len(ids) == 0 {
		return fmt.Errorf("scale-in aborted: no Nomad nodes were selected for removal "+
			"(requested %d, pool has %d running instances); check node eligibility and "+
			"that unique.platform.oci.instance_id is set on each OCI Nomad node",
			num, len(instanceOCIDs))
	}

	// Resolved once for both inter-detach RUNNING waits and the final convergence poll.
	retryAttempts, err := t.resolveRetryAttempts(config)
	if err != nil {
		return err
	}

	for i, node := range ids {
		ocid := node.RemoteResourceID
		log.Debug("detaching OCI instance", "ocid", ocid)
		if err := t.detachInstance(ctx, poolID, ocid); err != nil {
			return fmt.Errorf("failed to detach OCI instance %s: %w", ocid, err)
		}

		// OCI transitions to SCALING after each detach; wait for RUNNING before the next.
		if i < len(ids)-1 {
			interTarget := currentCount - int64(i+1)
			log.Info("waiting for pool to return to RUNNING before next detach",
				"after_detach", i+1, "interim_target", interTarget)
			if err := t.ensurePoolConverged(ctx, poolID, interTarget, retryAttempts); err != nil {
				return fmt.Errorf("pool did not return to RUNNING after detach %d: %w", i+1, err)
			}
		}
	}

	// Wait for convergence; without this, a second scale-in could start before this one settles.
	newTarget := currentCount - int64(len(ids))
	if err := t.ensurePoolConverged(ctx, poolID, newTarget, retryAttempts); err != nil {
		return fmt.Errorf("failed to confirm scale-in: %w", err)
	}

	log.Info("successfully scaled in OCI instance pool")

	if err := t.clusterUtils.RunPostScaleInTasks(ctx, config, ids); err != nil {
		return fmt.Errorf("failed to perform post-scale Nomad scale-in tasks: %w", err)
	}

	return nil
}

// listPoolInstances returns the OCIDs of all RUNNING instances in the pool,
// paging through the API response automatically.
func (t *TargetPlugin) listPoolInstances(ctx context.Context, poolID string, config map[string]string) ([]string, error) {
	compartmentID, ok := t.getValue(config, configKeyCompartmentID)
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyCompartmentID)
	}

	req := core.ListInstancePoolInstancesRequest{
		CompartmentId:  common.String(compartmentID),
		InstancePoolId: common.String(poolID),
	}

	var ocids []string
	for {
		resp, err := t.poolClient.ListInstancePoolInstances(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("ListInstancePoolInstances(%s): %w", poolID, err)
		}

		for i := range resp.Items {
			inst := resp.Items[i]
			if inst.Id == nil || inst.State == nil {
				continue
			}
			if strings.EqualFold(*inst.State, "running") {
				ocids = append(ocids, *inst.Id)
			}
		}

		if resp.OpcNextPage == nil {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return ocids, nil
}

// detachInstance detaches and terminates one OCI instance.
// isDecrementSize=true prevents OCI from spawning a replacement.
func (t *TargetPlugin) detachInstance(ctx context.Context, poolID, instanceID string) error {
	decrement := true
	autoTerminate := true

	_, err := t.poolClient.DetachInstancePoolInstance(ctx, core.DetachInstancePoolInstanceRequest{
		InstancePoolId: common.String(poolID),
		DetachInstancePoolInstanceDetails: core.DetachInstancePoolInstanceDetails{
			InstanceId:      common.String(instanceID),
			IsDecrementSize: &decrement,
			IsAutoTerminate: &autoTerminate,
		},
	})
	if err != nil {
		return fmt.Errorf("DetachInstancePoolInstance: %w", err)
	}

	return nil
}

// ensurePoolConverged polls until CurrentSize == target and state == RUNNING,
// or retryAttempts is exhausted.
func (t *TargetPlugin) ensurePoolConverged(ctx context.Context, poolID string, target int64, retryAttempts int) error {
	f := func(ctx context.Context) (bool, error) {
		count, ready, err := t.poolStatus(ctx, poolID)
		if err != nil {
			return true, err
		}
		if ready && count == target {
			return true, nil
		}
		return false, errors.New("waiting for OCI instance pool to converge")
	}

	t.logger.Info("waiting for OCI instance pool convergence",
		"pool_id", poolID,
		"target_size", target,
	)
	return retry(ctx, t.retryInterval, retryAttempts, f)
}

func (t *TargetPlugin) resolveRetryAttempts(config map[string]string) (int, error) {
	attempts := t.retryAttempts
	if str, ok := config[configKeyRetryAttempts]; ok {
		n, err := strconv.Atoi(str)
		if err != nil {
			return 0, fmt.Errorf("failed to parse %s from policy: %w", configKeyRetryAttempts, err)
		}
		attempts = n
	}
	return attempts, nil
}

// ociNodeIDMap maps a Nomad node to its OCI instance OCID via the node's
// attributes or meta. The key "unique.platform.oci.instance_id" must be set
// on each OCI VM (e.g. via Nomad client meta config or cloud-init).
func ociNodeIDMap(n *api.Node) (string, error) {
	if val, ok := n.Attributes[nodeAttrOCIInstanceID]; ok {
		return val, nil
	}
	if val, ok := n.Meta[nodeAttrOCIInstanceID]; ok {
		return val, nil
	}
	return "", fmt.Errorf("attribute %q not found on node %s; "+
		"ensure Nomad client config sets meta.%s = <instance-ocid>",
		nodeAttrOCIInstanceID, n.ID, nodeAttrOCIInstanceID)
}
