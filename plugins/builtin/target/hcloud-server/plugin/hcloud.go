package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
	"github.com/hashicorp/nomad/api"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

const (
	defaultRetryInterval   = 10 * time.Second
	defaultRetryLimit      = 5
	defaultPerPage         = 50
	defaultRandomSuffixLen = 10
	nodeAttrHCloudServerID = "unique.hostname"
)

// setupHCloudClient takes the passed config mapping and instantiates the
// required Hetzner Cloud client.
func (t *TargetPlugin) setupHCloudClient(config map[string]string) error {

	token, ok := config[configKeyToken]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyToken)
	}

	t.hcloud = hcloud.NewClient(hcloud.WithToken(token))
	return nil
}

// scaleOut adds HCloud servers up to desired count to match what the
// Autoscaler has deemed required.
func (t *TargetPlugin) scaleOut(ctx context.Context, servers []*hcloud.Server, count int64, config map[string]string) error {

	namePrefix, ok := config[configKeyNamePrefix]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyNamePrefix)
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_out", "hcloud_name_prefix", namePrefix,
		"desired_count", count)

	opts := hcloud.ServerCreateOpts{}

	datacenter, ok := config[configKeyDatacenter]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyDatacenter)
	} else {
		opts.Datacenter = &hcloud.Datacenter{Name: datacenter}
	}

	if location, ok := config[configKeyLocation]; ok {
		opts.Location = &hcloud.Location{Name: location}
	}

	imageName, ok := config[configKeyImage]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyImage)
	} else {
		image, _, err := t.hcloud.Image.Get(ctx, imageName)
		if err != nil {
			return fmt.Errorf("couldn't retrieve HCloud image: %v", err)
		}
		opts.Image = image
	}

	userData, ok := config[configKeyUserData]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyUserData)
	} else {
		opts.UserData = userData
	}

	sshKeys, ok := config[configKeySSHKeys]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeySSHKeys)
	} else {
		for _, sshKeyValue := range strings.Split(sshKeys, ",") {
			sshKey, _, err := t.hcloud.SSHKey.Get(ctx, sshKeyValue)
			if err != nil {
				return fmt.Errorf("failed to get HCloud SSH key: %v", err)
			}
			if sshKey == nil {
				return fmt.Errorf("HCloud SSH key not found: %s", sshKeyValue)
			}
			opts.SSHKeys = append(opts.SSHKeys, sshKey)
		}
	}

	labels, ok := config[configKeyLabels]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyLabels)
	} else {
		labelsResult, err := extractLabels(labels)
		if err != nil {
			return fmt.Errorf("failed to parse labels during instance scale_out: %v", err)
		}
		opts.Labels = labelsResult
	}

	f := func(ctx context.Context) (bool, error) {
		var results []hcloud.ServerCreateResult
		countDiff := count - int64(len(servers))
		var counter int64
		for counter < countDiff {
			id := uuid.New()
			suffix := strings.Replace(id.String(), "-", "", -1)[:defaultRandomSuffixLen]
			opts.Name = fmt.Sprintf("%s-%s", namePrefix, suffix)
			result, _, err := t.hcloud.Server.Create(ctx, opts)
			if err != nil {
				log.Error("failed to create an HCloud server: %v", err)
			}
			results = append(results, result)
		}
		var actionIDs []int
		for _, result := range results {
			if result.Action.Progress < 100 {
				actionIDs = append(actionIDs, result.Action.ID)
			}
		}
		_, _, err := t.ensureActionsComplete(ctx, actionIDs)
		if err != nil {
			log.Error("failed to wait till all HCloud create actions are ready: %v", err)
		}
		servers, err = t.getServers(ctx, labels)
		if err != nil {
			return false, fmt.Errorf("failed to get a new servers count during instance scale out: %v", err)
		}
		serverCount := int64(len(servers))
		if serverCount == count {
			return true, nil
		}
		return false, fmt.Errorf("waiting for %v servers to create", count-serverCount)
	}

	return retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
}

func (t *TargetPlugin) scaleIn(ctx context.Context, servers []*hcloud.Server, count int64, config map[string]string) error {

	if t.clusterUtils.ClusterNodeIDLookupFunc == nil {
		return errors.New("required ClusterNodeIDLookupFunc not set")
	}

	nodes, err := t.clusterUtils.IdentifyScaleInNodes(config, len(servers))
	if err != nil {
		return err
	}

	nodeResourceIDs, err := t.clusterUtils.IdentifyScaleInRemoteIDs(nodes)
	if err != nil {
		return err
	}

	// Any error received here indicates misconfiguration between the Hetzner servers and
	// the Nomad node pool.
	err = validateServers(servers, nodeResourceIDs)
	if err != nil {
		return err
	}

	if err := t.clusterUtils.DrainNodes(ctx, config, nodeResourceIDs); err != nil {
		return err
	}
	t.logger.Info("pre scale-in tasks now complete")

	namePrefix, ok := config[configKeyNamePrefix]
	if !ok {
		return fmt.Errorf("required config param %s not found", configKeyNamePrefix)
	}

	// Create a logger for this action to pre-populate useful information we
	// would like on all log lines.
	log := t.logger.With("action", "scale_in", "hcloud_name_prefix", namePrefix)

	var failedIDs, successfulIDs []scaleutils.NodeResourceID
	f := func(ctx context.Context) (bool, error) {
		var actionIDs map[int]scaleutils.NodeResourceID
		for _, node := range nodeResourceIDs {
			var id int
			for _, server := range servers {
				if server.Name == node.RemoteResourceID {
					id = server.ID
					break
				}
			}
			serverInput := hcloud.Server{
				ID: id,
			}
			resp, err := t.hcloud.Server.Delete(ctx, &serverInput)
			if err != nil {
				log.Error("failed to delete a HCloud server",
					"server_id", node.RemoteResourceID, "node_id", node.NomadNodeID,
					"error", err)
			}
			deleteServerJsonData, err := ioutil.ReadAll(resp.Body)
			var action hcloud.Action
			err = json.Unmarshal([]byte(deleteServerJsonData), &action)
			if err != nil {
				log.Error("failed to parse HCloud delete server action",
					"server_id", node.RemoteResourceID, "node_id", node.NomadNodeID,
					"error", err)
			}
			if action.Progress < 100 {
				actionIDs[action.ID] = node
			} else if action.Status == hcloud.ActionStatusError {
				failedIDs = append(failedIDs, node)
			} else if action.Status == hcloud.ActionStatusSuccess {
				successfulIDs = append(successfulIDs, node)
			}
		}
		actionIDsKeys := make([]int, 0, len(actionIDs))
		for actionID := range actionIDs {
			actionIDsKeys = append(actionIDsKeys, actionID)
		}

		successfulActions, failedActions, err := t.ensureActionsComplete(ctx, actionIDsKeys)
		if err != nil {
			t.logger.Error("failed to wait till all HCloud delete actions are complete: %v", err)
		}

		for _, successfulAction := range successfulActions {
			successfulIDs = append(successfulIDs, actionIDs[successfulAction])
		}

		for _, failedAction := range failedActions {
			failedIDs = append(failedIDs, actionIDs[failedAction])
		}

		if len(failedIDs) == 0 {
			return true, nil
		} else {
			nodeResourceIDs = failedIDs
			failedIDs = []scaleutils.NodeResourceID{}
		}

		return false, fmt.Errorf("waiting for %v servers to delete", len(servers))
	}

	err = retry(ctx, defaultRetryInterval, defaultRetryLimit, f)

	var failedTaskErr, successTaskErr error

	if len(failedIDs) > 0 {
		failedTaskErr = t.clusterUtils.RunPostScaleInTasksOnFailure(failedIDs)
	}

	if len(successfulIDs) > 0 {
		successTaskErr = t.clusterUtils.RunPostScaleInTasks(ctx, config, successfulIDs)
	}

	if successTaskErr != nil {
		t.logger.Error("failed to perform post-scale Nomad scale in tasks", "error", successTaskErr)
	}

	if len(failedIDs) > 0 && len(successfulIDs) > 0 {
		t.logger.Warn("partial scaling success",
			"success_num", len(successfulIDs), "failed_num", len(failedIDs),
			"error", failedTaskErr)
	}

	return err
}

// validateServers checks that all the instances identified for scaling in
// belong to the Hetzner nodes.
func validateServers(servers []*hcloud.Server, ids []scaleutils.NodeResourceID) error {

	// isMissing tracks the total number of instance deemed missing from the
	// ASG to provide some user context.
	var isMissing int

	for _, node := range ids {

		// found identifies whether this individual node has been located
		// within the ASG.
		var found bool

		// Iterate the instance within the ASG, and exit if we identify a
		// match to continue below.
		for _, server := range servers {
			if node.RemoteResourceID == server.Name {
				found = true
				break
			}
		}

		if !found {
			isMissing++
		}
	}

	if isMissing > 0 {
		return fmt.Errorf("%v selected nodes are not found among Hetzner Cloud nodes", isMissing)
	}
	return nil
}

func (t *TargetPlugin) getServers(ctx context.Context, labelSelector string) ([]*hcloud.Server, error) {
	opts := hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: labelSelector,
			PerPage:       defaultPerPage,
		},
		Status: []hcloud.ServerStatus{hcloud.ServerStatusRunning},
	}
	servers, err := t.hcloud.Server.AllWithOpts(ctx, opts)
	if err != nil {
		t.logger.Error("error retrieving server: %s\n", err)
		return nil, err
	}
	return servers, nil
}

func (t *TargetPlugin) ensureActionsComplete(ctx context.Context, ids []int) (successfulActions []int, failedActions []int, err error) {

	opts := hcloud.ActionListOpts{
		ID: ids,
	}

	f := func(ctx context.Context) (bool, error) {
		currentActions, _, err := t.hcloud.Action.List(ctx, opts)
		if err != nil {
			return true, err
		}

		// Reset the action IDs we are waiting to complete so we can
		// re-populate with a modified list later.
		var ids []int

		// Iterate each action, check the progress and add any incomplete
		// actions to the ID list for rechecking.
		for _, action := range currentActions {
			if action.Progress < 100 {
				ids = append(ids, action.ID)
			} else if action.Status == hcloud.ActionStatusError {
				failedActions = append(failedActions, action.ID)
				t.logger.Error("Hetzner cloud action id %v failed with code %v: %v", action.ID, action.ErrorCode, action.ErrorMessage)
			} else if action.Status == hcloud.ActionStatusSuccess {
				successfulActions = append(successfulActions, action.ID)
			}
		}

		// If we dont have any remaining IDs to check, we can finish.
		if len(ids) == 0 {
			return true, nil
		}
		return false, fmt.Errorf("waiting for %v actions to finish", len(ids))
	}

	err = retry(ctx, defaultRetryInterval, defaultRetryLimit, f)
	return
}

// hcloudNodeIDMap is used to identify the HCloud Server of a Nomad node using
// the relevant attribute value.
func hcloudNodeIDMap(n *api.Node) (string, error) {
	val, ok := n.Attributes[nodeAttrHCloudServerID]
	if !ok || val == "" {
		return "", fmt.Errorf("attribute %q not found", nodeAttrHCloudServerID)
	}
	return val, nil
}

func extractLabels(labelsStr string) (map[string]string, error) {
	var labels map[string]string
	labelStrs := strings.Split(labelsStr, ",")
	for _, labelStr := range labelStrs {
		labelValues := strings.Split(labelStr, "=")
		if len(labelValues) == 2 {
			labels[strings.TrimSpace(labelValues[0])] = strings.TrimSpace(labelValues[1])
		} else {
			return nil, fmt.Errorf("failed to parse labels: %s", labelsStr)
		}
	}
	return labels, nil
}
