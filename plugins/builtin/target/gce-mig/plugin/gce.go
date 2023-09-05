// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/go-homedir"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

const (
	defaultRetryInterval = 10 * time.Second
	defaultRetryLimit    = 15

	// nodeAttrGCEHostname is the node attribute to use when identifying the
	// GCE hostname of a node.
	nodeAttrGCEHostname = "unique.platform.gce.hostname"

	// nodeAttrGCEZone is the node attribute to use when identifying the GCE
	// zone of a node.
	nodeAttrGCEZone = "platform.gce.zone"
)

func (t *TargetPlugin) setupGCEClients(config map[string]string) error {

	credentials, ok := config[configKeyCredentials]
	if ok {
		contents, err := pathOrContents(credentials)
		if err != nil {
			return fmt.Errorf("failed to read credentials: %v", err)
		}

		t.service, err = compute.NewService(context.Background(), option.WithCredentialsJSON([]byte(contents)))
		if err != nil {
			return fmt.Errorf("failed to create Google Compute Engine client: %v", err)
		}
	} else {
		service, err := compute.NewService(context.Background())
		if err != nil {
			return fmt.Errorf("failed to create Google Compute Engine client: %v", err)
		}

		t.service = service
	}

	return nil
}

func (t *TargetPlugin) status(ctx context.Context, ig instanceGroup) (bool, int64, error) {
	return ig.status(ctx, t.service)
}

func (t *TargetPlugin) scaleOut(ctx context.Context, ig instanceGroup, num int64) error {
	log := t.logger.With("action", "scale_out", "instance_group", ig.getName())
	if err := ig.resize(ctx, t.service, num); err != nil {
		return fmt.Errorf("failed to scale out GCE Instance Group: %v", err)
	}
	if err := t.ensureInstanceGroupIsStable(ctx, ig); err != nil {
		return fmt.Errorf("failed to confirm scale out GCE Instance Group: %v", err)
	}
	log.Debug("scale out GCE MIG confirmed")
	return nil
}

func (t *TargetPlugin) scaleIn(ctx context.Context, group instanceGroup, num int64, config map[string]string) error {
	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "instance_group", group.getName())

	// Find instance IDs in the target instance group and perform pre-scale tasks.
	instances, err := group.listInstances(ctx, t.service)
	if err != nil {
		return fmt.Errorf("failed to list GCE MIG instances: %v", err)
	}

	remoteIDs := []string{}
	for _, inst := range instances {
		if inst.InstanceStatus == "RUNNING" && inst.CurrentAction == "NONE" {
			log.Debug("found healthy instance", "instance_id", inst.Id, "instance", inst.Instance)

			// Use the partial URL since that's what gceNodeIDMap returns.
			idx := strings.Index(inst.Instance, "/zones/")
			remoteIDs = append(remoteIDs, inst.Instance[idx+1:])
		} else {
			log.Debug("skipping instance", "instance_id", inst.Id, "instance", inst.Instance, "instance_status", inst.InstanceStatus, "current_action", inst.CurrentAction)
		}
	}

	ids, err := t.clusterUtils.RunPreScaleInTasksWithRemoteCheck(ctx, config, remoteIDs, int(num))
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
	}

	// Grab the instanceIDs
	var instanceIDs []string

	for _, node := range ids {
		instanceIDs = append(instanceIDs, node.RemoteResourceID)
	}

	// Delete the instances from the Managed Instance Groups. The targetSize of the MIG is will be reduced by the
	// number of instances that are deleted.
	log.Debug("deleting GCE MIG instances", "instances", ids)

	if err := group.deleteInstance(ctx, t.service, instanceIDs); err != nil {
		return fmt.Errorf("failed to delete instances: %v", err)
	}

	log.Info("successfully deleted GCE MIG instances")

	if err := t.ensureInstanceGroupIsStable(ctx, group); err != nil {
		return fmt.Errorf("failed to confirm scale in GCE MIG: %v", err)
	}

	log.Debug("scale in GCE MIG confirmed")

	// Run any post scale in tasks that are desired.
	if err := t.clusterUtils.RunPostScaleInTasks(ctx, config, ids); err != nil {
		return fmt.Errorf("failed to perform post-scale Nomad scale in tasks: %v", err)
	}

	return nil
}

func (t *TargetPlugin) ensureInstanceGroupIsStable(ctx context.Context, group instanceGroup) error {

	f := func(ctx context.Context) (bool, error) {
		stable, _, err := group.status(ctx, t.service)
		if stable || err != nil {
			return true, err
		} else {
			return false, errors.New("waiting for instance group to become stable")
		}
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func pathOrContents(poc string) (string, error) {
	if len(poc) == 0 {
		return poc, nil
	}

	path := poc
	if path[0] == '~' {
		var err error
		path, err = homedir.Expand(path)
		if err != nil {
			return path, err
		}
	}

	if _, err := os.Stat(path); err == nil {
		contents, err := os.ReadFile(path)
		if err != nil {
			return string(contents), err
		}
		return string(contents), nil
	}

	return poc, nil
}

// gceNodeIDMap is used to identify the GCE Instance of a Nomad node using the
// relevant attribute value.
func gceNodeIDMap(n *api.Node) (string, error) {
	zone, ok := n.Attributes[nodeAttrGCEZone]
	if !ok {
		return "", fmt.Errorf("attribute %q not found", nodeAttrGCEZone)
	}
	hostname, ok := n.Attributes[nodeAttrGCEHostname]
	if !ok {
		return "", fmt.Errorf("attribute %q not found", nodeAttrGCEHostname)
	}
	if idx := strings.Index(hostname, "."); idx != -1 {
		return fmt.Sprintf("zones/%s/instances/%s", zone, hostname[0:idx]), nil
	} else {
		return fmt.Sprintf("zones/%s/instances/%s", zone, hostname), nil
	}
}
