package ha

import (
	"context"

	"github.com/hashicorp/nomad-autoscaler/policy"
)

type MonitorFilterRequest struct {
	ErrCh    chan<- error
	UpdateCh chan<- struct{}
}

// PolicyFilter defines the interface for policy filters used by the
// autoscaler's HA capability.
type PolicyFilter interface {

	// MonitorFilterUpdates accepts a context and a channel for informing the
	// caller of asynchronous updates to the underlying filter.
	MonitorFilterUpdates(ctx context.Context, req MonitorFilterRequest)

	// ReloadFilterMonitor indicates that the filter should be reloaded due to
	// potential changes to configuration and/or clients.
	ReloadFilterMonitor()

	// FilterPolicies should return a list of policies appropriate for this
	// autoscaler agent; policies for other autoscaler agents in the HA pool
	// should be neglected from the returned slice.
	FilterPolicies(policyIDs []policy.PolicyID) []policy.PolicyID
}
