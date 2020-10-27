// +build !ent

package nomad

import "github.com/hashicorp/nomad-autoscaler/sdk"

func (s *Source) canonicalizeAdditionalTypes(p *sdk.ScalingPolicy) {}
