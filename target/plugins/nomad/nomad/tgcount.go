package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad-autoscaler/strategy"
	"github.com/hashicorp/nomad/api"
)

type NomadGroupCount struct {
	client *api.Client
}

func (t *NomadGroupCount) SetConfig(config map[string]string) error {
	clientConfig := api.DefaultConfig()

	if config["region"] != "" {
		clientConfig.Region = config["region"]
	}
	if config["address"] != "" {
		clientConfig.Address = config["address"]
	}

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	t.client = client
	return nil
}

func (t *NomadGroupCount) Count(config map[string]string) (int64, error) {
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

func (t *NomadGroupCount) Scale(action strategy.Action, config map[string]string) error {
	countInt := int(action.Count)
	_, _, err := t.client.Jobs().Scale(config["job_id"], config["group"], &countInt, &action.Reason, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
	}
	return nil
}
