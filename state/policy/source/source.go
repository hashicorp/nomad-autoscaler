package source

import "github.com/hashicorp/nomad/api"

// PolicySource is the interface that must be implemented by backends which
// provide the canonical source for scaling policies.
type PolicySource interface {

	// Start triggers the long lived process which monitors for policy changes
	// and sends updates to the provide channel for processing and updating
	// within the data store.
	Start(updateChan chan *api.ScalingPolicy)
}
