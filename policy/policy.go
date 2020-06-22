package policy

import (
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

type Policy struct {
	ID                 string
	Min                int64
	Max                int64
	Enabled            bool
	Cooldown           time.Duration
	EvaluationInterval time.Duration
	Checks             []*Check
	Target             *Target
}

type Check struct {
	Name     string    `hcl:"name,label"`
	Source   string    `hcl:"source"`
	Query    string    `hcl:"query"`
	Strategy *Strategy `hcl:"strategy,block"`
}

type Strategy struct {
	Name   string            `hcl:"name,label"`
	Config map[string]string `hcl:",remain"`
}

type Target struct {
	Name   string            `hcl:"name,label"`
	Config map[string]string `hcl:",remain"`
}

type Evaluation struct {
	Policy       *Policy
	TargetStatus *target.Status
}

// Apply applies the config defaults to the policy where the operator does not
// supply the parameter. This can be used for both cluster and task group
// policies.
func (p *Policy) ApplyDefaults(d *ConfigDefaults) {
	if p.Cooldown == 0 {
		p.Cooldown = d.DefaultCooldown
	}
	if p.EvaluationInterval == 0 {
		p.EvaluationInterval = d.DefaultEvaluationInterval
	}
}

// FileDecodePolicy is used as an intermediate step when decoding a policy from
// a file. It is needed because the internal Policy object is flattened when
// compared to the literal HCL version. Therefore we cannot translate into the
// internal struct but use this.
type FileDecodePolicy struct {
	Enabled bool                 `hcl:"enabled,optional"`
	Min     int64                `hcl:"min,optional"`
	Max     int64                `hcl:"max"`
	Doc     *FileDecodePolicyDoc `hcl:"policy,block"`
}

type FileDecodePolicyDoc struct {
	Cooldown              time.Duration
	CooldownHCL           string `hcl:"cooldown,optional"`
	EvaluationInterval    time.Duration
	EvaluationIntervalHCL string   `hcl:"evaluation_interval,optional"`
	Checks                []*Check `hcl:"check,block"`
	Target                *Target  `hcl:"target,block"`
}

// Translate all values from the decoded policy file into our internal policy
// object.
func (fpd *FileDecodePolicy) Translate(p *Policy) {
	p.Min = fpd.Min
	p.Max = fpd.Max
	p.Enabled = fpd.Enabled
	p.Cooldown = fpd.Doc.Cooldown
	p.EvaluationInterval = fpd.Doc.EvaluationInterval
	p.Checks = fpd.Doc.Checks
	p.Target = fpd.Doc.Target
}
