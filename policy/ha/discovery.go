package ha

import (
	"context"

	consulapi "github.com/hashicorp/consul/api"
)

type ConsulCatalogAPI interface {
	Datacenters() ([]string, error)
	QueryService(service, tag string, q *consulapi.QueryOptions) ([]*consulapi.CatalogService, *consulapi.QueryMeta, error)
	RegisterAgent(service, agentID string) error
	DeregisterAgent(service, agentID string) error
	UpdateConsulClient(consul *consulapi.Client)
}

type AgentID string

type DiscoveryRequest struct {
	updateCh chan<- []AgentID
	errCh    chan<- error
}

type PoolDiscovery interface {
	// AgentID returns the identifier of this agent. The implementation should
	// strive to make this consistent across restarts of this agent.
	// It is required to be unique across agent instances.
	AgentID() string

	// MonitorPool accepts a context and channels for informing the caller
	// of asynchronous updates to the pool.
	MonitorPool(ctx context.Context, req DiscoveryRequest)

	// RegisterAgent registers this agent in the pool.
	// Should makes a best-effort attempt to deregister the agent when the context
	// is closed.
	RegisterAgent(ctx context.Context) error

	// Reload instructs MonitorPool to reload the pool, as a result of
	// updated configuration.
	Reload()
}
