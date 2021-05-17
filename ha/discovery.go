package ha

import (
	"context"
)

// agentID is a basic type wrapper around the autoscaler agent ID which helps
// with assurance because this is critical to the HA capability.
type AgentID string

// discoveryRequest is used to monitor the pool of autoscaler agents and report
// on them.
type DiscoveryRequest struct {
	UpdateCh chan<- []AgentID
	ErrCh    chan<- error
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
	MonitorPool(ctx context.Context, req DiscoveryRequest)

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
