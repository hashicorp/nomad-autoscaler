package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadAPM "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/nomad/plugin"
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

// CanonicalizeAPMQuery takes a short styled Nomad APM check query and creates
// its fully hydrated internal representation. This is required by the Nomad
// APM if it is being used as the source. The function can be called without
// any validation on the check.
func (c *Check) CanonicalizeAPMQuery(t *Target) {

	// Catch nils so this function is safe to call without any prior checks.
	if c == nil || t == nil {
		return
	}

	// If the query source is not the Nomad APM, we do not have any additional
	// work to perform. The APM canonicalization is specific to the Nomad APM.
	if c.Source != plugins.InternalAPMNomad {
		return
	}

	// If the query is not formatted in the short manner we do not have any
	// work to do. Operators can add this if they want/know the autoscaler
	// internal model.
	if !isShortQuery(c.Query) {
		return
	}

	// If the target is a Nomad job task group, format the query in the
	// expected manner.
	if t.isJobTaskGroupTarget() {
		c.Query = fmt.Sprintf("%s_%s/%s/%s",
			nomadAPM.QueryTypeTaskGroup, c.Query, t.Config[target.ConfigKeyTaskGroup], t.Config[target.ConfigKeyJob])
		return
	}

	// If the target is a Nomad client node pool, format the query in the
	// expected manner. Once the autoscaler supports more than just class
	// identification of pools this func and logic will need to be updated. For
	// now keep it simple.
	if t.isNodePoolTarget() {
		c.Query = fmt.Sprintf("%s_%s/%s/class",
			nomadAPM.QueryTypeNode, c.Query, t.Config[target.ConfigKeyClass])
	}
}

func (t *Target) isJobTaskGroupTarget() bool {
	_, jOK := t.Config[target.ConfigKeyJob]
	_, gOK := t.Config[target.ConfigKeyTaskGroup]
	return jOK && gOK
}

func (t *Target) isNodePoolTarget() bool {
	_, ok := t.Config[target.ConfigKeyClass]
	return ok
}

// isShortQuery detects if a query is in the <type>_<op>_<metric> format which
// is required by the Nomad APM.
func isShortQuery(q string) bool {
	opMetric := strings.SplitN(q, "_", 2)
	hasSlash := strings.Contains(q, "/")
	return len(opMetric) == 2 && !hasSlash
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
