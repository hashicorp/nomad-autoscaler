package nomad

import "github.com/hashicorp/nomad/api"

func validateClusterPolicy(policy *api.ScalingPolicy) error {
	// Use the same validation rules for now.
	return validateHorizontalPolicy(policy)
}
