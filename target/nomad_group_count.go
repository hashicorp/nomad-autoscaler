package target

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad-autoscaler/strategy"
	"github.com/hashicorp/nomad/api"
)

type NomadGroupCount struct{}

func (t *NomadGroupCount) Count(config map[string]string) (int, error) {
	client, err := nomadClient(config)
	if err != nil {
		return 0, err
	}

	var count int
	allocs, _, err := client.Jobs().Allocations(config["job_id"], false, nil)
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

func (t *NomadGroupCount) Scale(actions []strategy.Action, config map[string]string) error {
	client, err := nomadClient(config)
	if err != nil {
		return err
	}

	for _, a := range actions {
		log.Printf("Scaled job %s/%s to %d. Reason: %s\n", config["job_id"], config["group"], a.Count, a.Reason)
		_, _, err = client.Jobs().Scale(config["job_id"], config["group"], a.Count, a.Reason, nil)
		if err != nil {
			return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
		}

	}
	return nil
}

func nomadClient(config map[string]string) (*api.Client, error) {
	clientConfig := api.DefaultConfig()
	clientConfig = clientConfig.ClientConfig(config["region"], config["address"], false)

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	return client, nil
}
