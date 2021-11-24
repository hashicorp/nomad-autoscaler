//go:build !ent
// +build !ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

func additionalPolicyTypeValidation(policy *api.ScalingPolicy) error {
	return fmt.Errorf("policy type %q not supported", policy.Type)
}
