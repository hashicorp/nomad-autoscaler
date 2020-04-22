package source

import "github.com/hashicorp/nomad/api"

// PolicySource is the interface that must be implemented by backends which
// provide the canonical source for scaling policies.
type PolicySource interface {

	// Start triggers the long lived process which monitors for policy changes
	// and sends updates to the provide channel for processing and updating
	// within the data store.
	//
	// The reconcileChan is used to perform reconciliation efforts when a list
	// of policies is received. The channel receiver is responsible for
	// comparing the current internal state with the provided list and removing
	// policies where necessary. This helps during purge, stop and such events.
	Start(updateChan chan *api.ScalingPolicy, reconcileChan chan []*api.ScalingPolicyListStub)
}
