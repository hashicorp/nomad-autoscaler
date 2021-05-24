package ha

import (
	"context"
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/hash"
	"github.com/hashicorp/nomad/api"
)

type PolicyFilter struct {
	discovery PoolDiscovery
	hashRing  *hash.ConsistentRing
}

func NewPolicyFilter(discovery PoolDiscovery) policy.PolicyFilter {
	return &PolicyFilter{
		discovery: discovery,
		hashRing:  hash.NewConsistentRing(),
	}
}

func (ch *PolicyFilter) SetNomadClient(nomad *api.Client) {
	if n, ok := ch.discovery.(policy.NomadClientUser); ok {
		n.SetNomadClient(nomad)
	}
}

func (ch *PolicyFilter) SetConsulClient(consul *consulapi.Client) {
	if c, ok := ch.discovery.(policy.ConsulClientUser); ok {
		c.SetConsulClient(consul)
	}
}

func (ch *PolicyFilter) MonitorFilterUpdates(ctx context.Context, req policy.MonitorFilterRequest) {
	updateCh := make(chan []hash.RingMember)
	errCh := make(chan error)

	go ch.discovery.MonitorPool(ctx, DiscoveryRequest{UpdateCh: updateCh, ErrCh: errCh})

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			req.ErrCh <- fmt.Errorf("filter received error channel from MonitorPool: %v", err)
			continue
		case update := <-updateCh:
			ch.hashRing.Update(update)
			req.UpdateCh <- struct{}{}
		}
	}
}

func (ch *PolicyFilter) ReloadFilterMonitor() { ch.discovery.Reload() }

func (ch *PolicyFilter) FilterPolicies(policyIDs []policy.PolicyID) []policy.PolicyID {
	return ch.hashRing.GetAssignments(policyIDs, ch.discovery.AgentID())
}
