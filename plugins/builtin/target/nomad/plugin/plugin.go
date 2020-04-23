package nomad

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nomad-target"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeTarget,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewNomadPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeTarget,
	}
)

type TargetPlugin struct {
	client *api.Client
	logger hclog.Logger
}

func NewNomadPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger: log,
	}
}

func (t *TargetPlugin) SetConfig(config map[string]string) error {

	cfg := nomadHelper.ConfigFromMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	t.client = client

	return nil
}

func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (t *TargetPlugin) Count(config map[string]string) (int64, error) {
	// TODO: validate if group is valid
	allocs, _, err := t.client.Jobs().Allocations(config["job_id"], false, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve Nomad job: %v", err)
	}

	var count int64
	for _, alloc := range allocs {
		if alloc.TaskGroup == config["group"] && alloc.ClientStatus == "running" {
			count++
		}
	}

	return count, nil
}

func (t *TargetPlugin) Scale(action strategy.Action, config map[string]string) error {
	var countIntPtr *int
	if action.Count != nil {
		countInt := int(*action.Count)
		countIntPtr = &countInt
	}

	_, _, err := t.client.Jobs().Scale(config["job_id"],
		config["group"],
		countIntPtr,
		action.Reason,
		action.Error,
		action.Meta,
		nil)

	if err != nil {
		return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
	}
	return nil
}

func (t *TargetPlugin) Status(config map[string]string) (*target.Status, error) {
	status, _, err := t.client.Jobs().ScaleStatus(config["job_id"], nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// Job doesn't exist anymore.
			return nil, nil
		}
		return nil, err
	}

	var count int
	for name, tg := range status.TaskGroups {
		if name == config["group"] {
			//TODO(luiz): not sure if this is the right value
			count = tg.Running
			break
		}
	}

	return &target.Status{
		Ready: !status.JobStopped,
		Count: int64(count),
	}, nil
}
