package ha

import (
	"context"

	consulapi "github.com/hashicorp/consul/api"
)

// ConsulCatalogAPI is an interface that wraps useful Consul catalog
// functionality for use within PoolDiscovery.
type ConsulCatalogAPI interface {

	// Datacenters lists the known Consul datacenters which provides a basis to
	// perform additional discovery.
	Datacenters() ([]string, error)

	// QueryService performs a Consul catalog query for the service with the
	// specified tag.
	QueryService(service, tag string, q *consulapi.QueryOptions) ([]*consulapi.CatalogService, *consulapi.QueryMeta, error)

	// RegisterAgent registers the autoscaler agent in Consul as a service with
	// an accompanying check. During the registration a unique ID is either
	// discovered or generated. If the agent is running as a Nomad allocation,
	// the allocation ID is used. Otherwise, a UUID is generated and persisted
	// to disk allowing restart survivability.
	RegisterAgent(service, agentID, agentAddr string, agentPort int) error

	// DeregisterAgent removes the autoscaler agent's service and checks from
	// Consul. In order to not orphan resources, it should be ensured this call
	// completes in one way, or another.
	DeregisterAgent(service, agentID string) error

	// UpdateConsulClient is used to update the Consul client used when making
	// API calls.
	UpdateConsulClient(consul *consulapi.Client)
}

// agentID is a basic type wrapper around the autoscaler agent ID which helps
// with assurance because this is critical to the HA capability.
type agentID string

// discoveryRequest is used to monitor the pool of autoscaler agents and report
// on them.
type discoveryRequest struct {
	updateCh chan<- []agentID
	errCh    chan<- error
}

// PoolDiscovery is the interface which describes the behaviour of a backend
// which supports the autoscaler's HA capabilities. The functionality includes
// discovering and monitoring other agents as well as describing details about
// the agent's self.
type PoolDiscovery interface {

	// AgentID returns the identifier of this agent. The implementation should
	// strive to make this consistent across restarts of this agent. It is
	// required to be unique across agent instances.
	AgentID() string

	// MonitorPool accepts a context and channels for informing the caller of
	// asynchronous updates to the pool.
	MonitorPool(ctx context.Context, req discoveryRequest)

	// RegisterAgent registers this agent in the pool. Should makes a
	// best-effort attempt to deregister the agent when the context is closed.
	RegisterAgent(ctx context.Context) error

	// Reload instructs MonitorPool to reload the pool, as a result of updated
	// configuration.
	Reload()

	// WaitForExit provides a blocking function that can be called when
	// ensuring the PoolDiscovery implementation has shut down properly. It is
	// useful where shutdown tasks must/should be completed.
	WaitForExit()
}
