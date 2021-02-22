package scaleutils

import "fmt"

// Ready provides a method for understanding whether the node pool is in a
// state that allows it to be safely scaled. This should be used by target
// plugins when providing their status response. A non-nil error indicates
// there was a problem performing the check.
func (si *ScaleIn) Ready(id PoolIdentifier) (bool, error) {

	nodes, _, err := si.nomad.Nodes().List(nil)
	if err != nil {
		return false, fmt.Errorf("failed to list Nomad nodes: %v", err)
	}

	// Validate the request object so we can differentiate between errors here
	// and errors as a result of node inconsistencies.
	if err := id.Validate(); err != nil {
		return false, fmt.Errorf("failed to validate pool identifier: %v", err)
	}

	if _, err := id.IdentifyNodes(nodes); err != nil {
		si.log.Warn("node pool status readiness check failed", "error", err)
		return false, nil
	}
	return true, nil
}
