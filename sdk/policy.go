package sdk

import "time"

// ScalingPolicy is the internal representation of a scaling document and
// encompasses all the required information for the autoscaler to perform
// scaling evaluations on a target.
type ScalingPolicy struct {

	// ID is a UUID which uniquely identifies this scaling policy. Depending on
	// the policy source this will be sourced in different manners.
	ID string

	// Min forms a lower bound at which the target should never be asked to
	// break. The autoscaler will actively adjust recommendations to ensure
	// this value is not violated.
	Min int64

	// Max forms an upper bound at which the target should never be asked to
	// exceed. The autoscaler will actively adjust recommendations to ensure
	// this value is not violated.
	Max int64

	// Enabled indicates whether the autoscaler should actively evaluate the
	// policy or not.
	Enabled bool

	// Cooldown is the time period after a scaling action if performed, during
	// which no policy evaluations will be started.
	Cooldown time.Duration

	// EvaluationInterval indicates the frequency at which the policy is
	// evaluated. A lower value means more frequent evaluation and can result
	// in a high rate of change in the target.
	EvaluationInterval time.Duration

	// Checks is an array of checks which will be triggered in parallel to
	// determine the desired state of the ScalingPolicyTarget.
	Checks []*ScalingPolicyCheck

	// Target identifies the scaling target which the autoscaler will interact
	// with to ensure it meets the desired state as determined by the Checks.
	Target *ScalingPolicyTarget
}

// ScalingPolicyCheck is an individual check within a scaling policy.This check
// will be executed in isolation alongside other checks within the policy.
type ScalingPolicyCheck struct {

	// Name is a human readable name for this check and allows operators to
	// create clearly identified policy checks.
	Name string

	// Source is the APM plugin that should be used to perform the query and
	// obtain the metric that will be used to perform a calculation.
	Source string

	// Query is run against the Source in order to receive a metric response.
	Query string

	// QueryWindow is used to define how further back in time to query for
	// metrics.
	QueryWindow time.Duration

	// Strategy is the ScalingPolicyStrategy to use when performing the
	// ScalingPolicyCheck evaluation.
	Strategy *ScalingPolicyStrategy
}

// ScalingPolicyStrategy contains the plugin and configuration details for
// calculating the desired target state from the current state.
type ScalingPolicyStrategy struct {

	// Name is the strategy that will be used to perform the desired state
	// calculation.
	Name string `hcl:"name,label"`

	// Config is the mapping of config values used by the strategy plugin. Each
	// plugin has a set of potentially uniquely supported keys.
	Config map[string]string `hcl:",remain"`
}

// ScalingPolicyTarget identifies the target for which the ScalingPolicy as a
// whole is configured for.
type ScalingPolicyTarget struct {

	// Name identifies the target plugin which can handle performing target
	// requests for this ScalingPolicy.
	Name string `hcl:"name,label"`

	// Config is the mapping of config values used by the target plugin. Each
	// plugin has a set of potentially uniquely supported keys.
	Config map[string]string `hcl:",remain"`
}

// IsJobTaskGroupTarget identifies whether the ScalingPolicyTarget relates to a
// Nomad job group.
func (t *ScalingPolicyTarget) IsJobTaskGroupTarget() bool {
	_, jOK := t.Config[TargetConfigKeyJob]
	_, gOK := t.Config[TargetConfigKeyTaskGroup]
	return jOK && gOK
}

// IsNodePoolTarget identifies whether the ScalingPolicyTarget relates to Nomad
// client nodes and therefore horizontal cluster scaling.
func (t *ScalingPolicyTarget) IsNodePoolTarget() bool {
	_, ok := t.Config[TargetConfigKeyClass]
	return ok
}

// FileDecodeScalingPolicy is used as an intermediate step when decoding a
// policy from a file. It is needed because the internal Policy object is
// flattened when compared to the literal HCL version. Therefore we cannot
// translate into the internal struct but use this.
type FileDecodeScalingPolicy struct {
	Enabled bool                 `hcl:"enabled,optional"`
	Min     int64                `hcl:"min,optional"`
	Max     int64                `hcl:"max"`
	Doc     *FileDecodePolicyDoc `hcl:"policy,block"`
}

type FileDecodePolicyDoc struct {
	Cooldown              time.Duration
	CooldownHCL           string `hcl:"cooldown,optional"`
	EvaluationInterval    time.Duration
	EvaluationIntervalHCL string                      `hcl:"evaluation_interval,optional"`
	Checks                []*FileDecodePolicyCheckDoc `hcl:"check,block"`
	Target                *ScalingPolicyTarget        `hcl:"target,block"`
}

type FileDecodePolicyCheckDoc struct {
	Name           string `hcl:"name,label"`
	Source         string `hcl:"source,optional"`
	Query          string `hcl:"query"`
	QueryWindow    time.Duration
	QueryWindowHCL string                 `hcl:"query_window,optional"`
	Strategy       *ScalingPolicyStrategy `hcl:"strategy,block"`
}

// Translate all values from the decoded policy file into our internal policy
// object.
func (fpd *FileDecodeScalingPolicy) Translate(p *ScalingPolicy) {
	p.Min = fpd.Min
	p.Max = fpd.Max
	p.Enabled = fpd.Enabled
	p.Cooldown = fpd.Doc.Cooldown
	p.EvaluationInterval = fpd.Doc.EvaluationInterval
	p.Target = fpd.Doc.Target

	fpd.translateChecks(p)
}

func (fpd *FileDecodeScalingPolicy) translateChecks(p *ScalingPolicy) {
	var checks []*ScalingPolicyCheck
	for _, c := range fpd.Doc.Checks {
		check := &ScalingPolicyCheck{}
		c.Translate(check)
		checks = append(checks, check)
	}

	p.Checks = checks
}

// Translate all values from the decoded policy check into our internal policy
// check object.
func (fdc *FileDecodePolicyCheckDoc) Translate(c *ScalingPolicyCheck) {
	c.Name = fdc.Name
	c.Source = fdc.Source
	c.Query = fdc.Query
	c.QueryWindow = fdc.QueryWindow
	c.Strategy = fdc.Strategy
}
