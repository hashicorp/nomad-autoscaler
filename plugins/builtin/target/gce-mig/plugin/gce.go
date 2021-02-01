package plugin

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/mitchellh/go-homedir"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
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

func (t *TargetPlugin) status(ctx context.Context, mr instanceGroup) (bool, int64, error) {
	return mr.status(ctx, t.service)
}

func (t *TargetPlugin) scaleOut(ctx context.Context, mr instanceGroup, num int64) error {
	return mr.resize(ctx, t.service, num)
}

func (t *TargetPlugin) scaleIn(ctx context.Context, mr instanceGroup, num int64, config map[string]string) error {
	scaleReq, err := t.generateScaleReq(num, config)
	if err != nil {
		return fmt.Errorf("failed to generate scale in request: %v", err)
	}

	ids, err := t.scaleInUtils.RunPreScaleInTasks(ctx, scaleReq)
	if err != nil {
		return fmt.Errorf("failed to perform pre-scale Nomad scale in tasks: %v", err)
	}

	// Grab the instanceIDs
	var instanceIDs []string

	for _, node := range ids {
		instanceIDs = append(instanceIDs, node.RemoteID)
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "instance_group", mr.getName(), "instances", ids)

	// Delete the instances from the Managed Instance Groups. The targetSize of the MIG is will be reduced by the
	// number of instances that are deleted.
	log.Debug("deleting GCE MIG instances")

	err = mr.deleteInstance(ctx, t.service, instanceIDs)
	if err != nil {
		return fmt.Errorf("failed to delete instances: %v", err)
	}

	log.Info("successfully deleted GCE MIG instances")

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
