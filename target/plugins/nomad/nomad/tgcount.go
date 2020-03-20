package nomad

import (
	"fmt"

	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/strategy"
	"github.com/hashicorp/nomad/api"
)

type NomadGroupCount struct {
	client *api.Client
}

func (t *NomadGroupCount) SetConfig(config map[string]string) error {

	cfg := nomadHelper.ConfigFromMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	t.client = client

	return nil
}

func (t *NomadGroupCount) Count(config map[string]string) (int, error) {
	var count int
	allocs, _, err := t.client.Jobs().Allocations(config["job_id"], false, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve Nomad job: %v", err)
	}

	for _, alloc := range allocs {
		if alloc.TaskGroup == config["group"] && alloc.ClientStatus == "running" {
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("group %s not found", config["group"])
	}

	return count, nil
}

func (t *NomadGroupCount) Scale(action strategy.Action, config map[string]string) error {
	_, _, err := t.client.Jobs().Scale(config["job_id"], config["group"], action.Count, &action.Reason, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
	}
	return nil
}
