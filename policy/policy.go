package policy

import (
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

type Policy struct {
	ID                 string
	Min                int64
	Max                int64
	Source             string
	Query              string
	Enabled            bool
	Cooldown           time.Duration
	EvaluationInterval time.Duration
	Target             *Target
	Strategy           *Strategy
}

type Strategy struct {
	Name   string
	Config map[string]string
}

type Target struct {
	Name   string
	Config map[string]string
}

type Evaluation struct {
	Policy       *Policy
	TargetStatus *target.Status
}
