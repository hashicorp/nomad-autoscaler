package ha

import (
	"context"
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/nomad-autoscaler/policy"
)

type ConsistentHashPolicyFilter struct {
	discovery   PoolDiscovery
	currentPool []AgentID
}

func (ch *ConsistentHashPolicyFilter) SetNomadClient(nomad *api.Client) {
	if n, ok := ch.discovery.(policy.NomadClientUser); ok {
		n.SetNomadClient(nomad)
	}
}

func (ch *ConsistentHashPolicyFilter) SetConsulClient(consul *consulapi.Client) {
	if c, ok := ch.discovery.(policy.ConsulClientUser); ok {
		c.SetConsulClient(consul)
	}
}

func (ch *ConsistentHashPolicyFilter) MonitorFilterUpdates(ctx context.Context, req MonitorFilterRequest) {
	updateCh := make(chan []AgentID)
	errCh := make(chan error)
	go ch.discovery.MonitorPool(ctx, DiscoveryRequest{
		updateCh: updateCh,
		errCh:    errCh,
	})
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			req.ErrCh <- fmt.Errorf("filter received error channel from MonitorPool: %v", err)
			continue
		case update := <-updateCh:
			ch.currentPool = update
			req.UpdateCh <- struct{}{}
		}
	}
}

func (ch *ConsistentHashPolicyFilter) ReloadFilterMonitor() {
	ch.discovery.Reload()
}

func (ch *ConsistentHashPolicyFilter) FilterPolicies(policyIDs []policy.PolicyID) []policy.PolicyID {
	return chPartition(ch.discovery.AgentID(), ch.currentPool, policyIDs)
}

func chPartition(myAgentID string, pool []AgentID, policies []policy.PolicyID) []policy.PolicyID {
	// FINISH: implement consistent hashing :P
	return nil
}

func NewConsistentHashPolicyFilter(discovery PoolDiscovery) PolicyFilter {
	return &ConsistentHashPolicyFilter{
		discovery: discovery,
	}
}
