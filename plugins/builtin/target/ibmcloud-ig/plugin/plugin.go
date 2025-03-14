// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "ibmcloud-ig"

	configKeyInstanceGroupID = "instance_group_id"
	configKeyAPIKey          = "api_key"
)

var (
	// this should catch IBMCloudIG not fulfilling the contract of the Target plugin as a
  // build-time failure.  If maintenance removes or fails to keep up, it's caught here
  // in a predictable place.
	_ target.Target = (*IBMCloudIG)(nil)

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewIBMIGPlugin(l) },
	}
)

type IBMCloudIG struct {
	config map[string]string
	logger hclog.Logger
	vpc    *vpcv1.VpcV1
}

// NewAWSASGPlugin returns the IBMCloud Instance Group implementation of the target.Target
// interface.
func NewIBMIGPlugin(log hclog.Logger) *IBMCloudIG {
	return &IBMCloudIG{
		logger: log,
	}
}

func (n *IBMCloudIG) Scale(action sdk.ScalingAction, config map[string]string) error {
	n.logger.Debug("received scale action", "count", action.Count, "reason", action.Reason)

	// early quit if we're in dry-run mode
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	// We cannot get the instance group from the name, we only have an API to get from ID
	instanceGroupID, ok := n.getConfig(configKeyInstanceGroupID, config)
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyInstanceGroupID)
	}

	apiKey, ok := n.getConfig(configKeyAPIKey, config)
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyAPIKey)
	}

	// ensure we have an authenticated lazy-defined client handle
	if n.vpc == nil {
		vpc, verr := vpcv1.NewVpcV1(
			&vpcv1.VpcV1Options{
				Authenticator: &core.IamAuthenticator{ApiKey: apiKey},
			})
		if verr != nil {
			return fmt.Errorf("error building authenticated client handle: %w", verr)
		}
		n.vpc = vpc
	}

	// hit that client handle to query information about the victim Instance Group
	ig, _, err := n.vpc.GetInstanceGroup(&vpcv1.GetInstanceGroupOptions{ID: &instanceGroupID})
	if err != nil {
		return fmt.Errorf("failed to GetInstanceGroup for ID %s: %w", instanceGroupID, err)
	}

	n.logger.Info(
		"ScaleAction: Instance Group status successfully queried",
		"instance_group_name", *ig.Name,
		"status",              *ig.Status,
		"size",                *ig.MembershipCount,
	)

	// Prepare a patch to set the new size
	instanceGroupPatchModel := vpcv1.InstanceGroupPatch{}
	instanceGroupPatchModel.MembershipCount = core.Int64Ptr(int64(action.Count))
	instanceGroupPatch, err := instanceGroupPatchModel.AsPatch()
	if err != nil {
		return fmt.Errorf("error creating patch for instance group %s: %w", instanceGroupID, err)
	}

	// Apply the patch
	options := &vpcv1.UpdateInstanceGroupOptions{}
	options.SetID(*ig.ID)
	options.InstanceGroupPatch = instanceGroupPatch

	// instanceGroup, response, err := vpc.UpdateInstanceGroup(...)
	_, _, uerr := n.vpc.UpdateInstanceGroup(options)
	if uerr != nil {
		return fmt.Errorf("error updating Instance Group with ID %s: %w", instanceGroupID, uerr)
	}

	// Victory Lap -- Maybe this can be removed to cut traffic to logfiles
	n.logger.Info(
		fmt.Sprintf(
			"ASG %s (%s) was scaled to %d using the IBM Cloud API",
			instanceGroupID, *ig.Name, action.Count,
		),
	)

	return nil
}

func (n *IBMCloudIG) Status(config map[string]string) (*sdk.TargetStatus, error) {
	instanceGroupID, ok := n.getConfig(configKeyInstanceGroupID, config)
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyInstanceGroupID)
	}

	apiKey, ok := n.getConfig(configKeyAPIKey, config)
	if !ok {
		return nil, fmt.Errorf("required config param %s not found", configKeyAPIKey)
	}

	// ensure we have a lazy-defined client handle
	if n.vpc == nil {
		vpc, verr := vpcv1.NewVpcV1(
			&vpcv1.VpcV1Options{
				Authenticator: &core.IamAuthenticator{ApiKey: apiKey},
			})
		if verr != nil {
			return nil, fmt.Errorf("error building authenticated client handle: %w", verr)
		}
		n.vpc = vpc
	}

	// hit that client handle to query information about the victim Instance Group
	ig, _, err := n.vpc.GetInstanceGroup(&vpcv1.GetInstanceGroupOptions{ID: &instanceGroupID})
	if err != nil {
		return nil, fmt.Errorf("failed to GetInstanceGroup for ID %s: %w", instanceGroupID, err)
	}

	n.logger.Info(
		"Instance Group status successfully queried",
		"instance_group_name", *ig.Name,
		"status",              *ig.Status,
		"size",                *ig.MembershipCount,
	)

	return &sdk.TargetStatus{
		Ready: (*ig.Status == vpcv1.InstanceGroupStatusHealthyConst),
		Count: int64(*ig.MembershipCount),
		Meta:  make(map[string]string),
	}, nil
}

func (n *IBMCloudIG) PluginInfo() (*base.PluginInfo, error) {
	return &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}, nil
}

func (n *IBMCloudIG) SetConfig(config map[string]string) error {
	n.config = config
	return nil
}

// getConfig checks the given config map first, falling-back to the object's configs.
func (n *IBMCloudIG) getConfig(key string, config map[string]string) (string, bool) {
	if ret, ok := config[key]; ok {
		return ret, ok
	}
	if ret, ok := n.config[key]; ok {
		return ret, ok
	}

	return "", false
}
