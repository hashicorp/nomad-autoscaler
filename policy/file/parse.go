package file

import (
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func decodeFile(file string, p *sdk.ScalingPolicy) error {

	decodePolicy := &sdk.FileDecodeScalingPolicy{}

	if err := hclsimple.DecodeFile(file, nil, decodePolicy); err != nil {
		return err
	}

	if decodePolicy.Doc.CooldownHCL != "" {
		d, err := time.ParseDuration(decodePolicy.Doc.CooldownHCL)
		if err != nil {
			return err
		}
		decodePolicy.Doc.Cooldown = d
	}

	if decodePolicy.Doc.EvaluationIntervalHCL != "" {
		d, err := time.ParseDuration(decodePolicy.Doc.EvaluationIntervalHCL)
		if err != nil {
			return err
		}
		decodePolicy.Doc.EvaluationInterval = d
	}

	// Translate from our intermediate struct, to our internal flattened
	// policy.
	decodePolicy.Translate(p)

	return nil
}
