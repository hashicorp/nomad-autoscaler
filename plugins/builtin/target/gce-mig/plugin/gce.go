package plugin

import (
	"context"
	"fmt"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/mitchellh/go-homedir"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"io/ioutil"
	"os"
	"time"
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

func (t *TargetPlugin) scaleOut(ctx context.Context, project string, zone string, mig string, num int64) error {

	_, err := t.service.RegionInstanceGroupManagers.Resize(project, zone, mig, num).Context(ctx).Do()

	if err != nil {
		return fmt.Errorf("failed to scale out instance group: %v", err)
	}

	return nil
}

func (t *TargetPlugin) scaleIn(ctx context.Context, project string, region string, mig string, num int64, config map[string]string) error {
	scaleReq, err := t.generateScaleReq(num, config)
	if err != nil {
		return fmt.Errorf("failed to generate scale in request: %v", err)
	}

	ids, err := t.scaleInUtils.RunPreScaleInTasks(ctx, scaleReq)
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
	}

	// Grab the instanceIDs once as it is used multiple times throughout the
	// scale in event.
	var instanceIDs []string

	for _, node := range ids {
		instanceIDs = append(instanceIDs, node.RemoteID)
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "instance_group", mig, "instances", ids)

	// Terminate the detached instances.
	log.Debug("deleting gce MIG instances")

	request := &compute.RegionInstanceGroupManagersDeleteInstancesRequest{
		Instances: instanceIDs,
	}

	_, err = t.service.RegionInstanceGroupManagers.DeleteInstances(project, region, mig, request).Context(ctx).Do()

	if err != nil {
		return fmt.Errorf("failed to delete instances: %v", err)
	}

	log.Info("successfully deleted gce MIG instances")

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

	if drainString, ok := config[sdk.TargetConfigKeyDrainDeadline]; ok {
		d, err := time.ParseDuration(drainString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q as time duration", drainString)
		}
		drain = d
	}

	return &scaleutils.ScaleInReq{
		Num:           int(num),
		DrainDeadline: drain,
		PoolIdentifier: &scaleutils.PoolIdentifier{
			IdentifierKey: scaleutils.IdentifierKeyClass,
			Value:         class,
		},
		RemoteProvider: scaleutils.RemoteProviderGCEInstanceID,
		NodeIDStrategy: scaleutils.IDStrategyNewestCreateIndex,
	}, nil
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
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			return string(contents), err
		}
		return string(contents), nil
	}

	return poc, nil
}
