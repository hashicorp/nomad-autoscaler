package policy

import "context"

// Source is the interface that must be implemented by backends which
// provide the canonical source for scaling policies.
type Source interface {

	// Start triggers the long lived process which monitors for policy changes
	// and sends updates to the provide channel for processing and updating
	// within the data store.
	//
	// The reconcileChan is used to perform reconciliation efforts when a list
	// of policies is received. The channel receiver is responsible for
	// comparing the current internal state with the provided list and removing
	// policies where necessary. This helps during purge, stop and such events.
	// Start(ctx context.Context, ch chan<- []PolicyID)

	MonitorIDs(ctx context.Context, resultCh chan<- []PolicyID, errCh chan<- error)
	MonitorPolicy(ctx context.Context, ID PolicyID, resultCh chan<- Policy, errCh chan<- error)
}

type PolicyID string
