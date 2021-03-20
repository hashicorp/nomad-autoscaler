package config

import "github.com/hashicorp/nomad-autoscaler/sdk/helper/modes"

// Modes stores the different capability modes of the Nomad Autoscaler.
var Modes = map[string]string{
	"ent": "Nomad Autoscaler Enterprise",
}

// NewModeChecker returns a new mode checker.
func NewModeChecker() *modes.Checker {
	return modes.NewChecker(Modes, ModesEnabled)
}
