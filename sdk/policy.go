// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sdk

import (
	"errors"
	"fmt"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
)

const (
	ScalingPolicyTypeCluster     = "cluster"
	ScalingPolicyTypeHorizontal  = "horizontal"
	ScalingPolicyTypeVerticalCPU = "vertical_cpu"
	ScalingPolicyTypeVerticalMem = "vertical_mem"

	ScalingPolicyOnErrorFail   = "fail"
	ScalingPolicyOnErrorIgnore = "ignore"
)

// ScalingPolicy is the internal representation of a scaling document and
// encompasses all the required information for the autoscaler to perform
// scaling evaluations on a target.
type ScalingPolicy struct {

	// ID is a UUID which uniquely identifies this scaling policy. Depending on
	// the policy source this will be sourced in different manners.
	ID string

	// Type is the type of scaling this policy will perform.
	Type string

	// Priority controls the order in which a policy is picked for evaluation.
	Priority int

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

	// OnCheckError defines how errors are handled by the Autoscaler when
	// running the policy checks. Possible values are "ignore" or "fail".
	//
	// If "ignore" the policy evaluation will continue even if a check fails.
	// If "fail" the the entire policy evaluation will stop and no action will
	// be taken.
	OnCheckError string

	// Cooldown is the time period after a scaling action is performed, during
	// which no policy evaluations will be started.
	Cooldown time.Duration

	// CooldownOnScaleUp is the time period after a scaling up action
	// is performed, during which no policy evaluations will be started. It is
	// as a separate option to allow for more aggressive scale up in case of
	// surges.
	CooldownOnScaleUp time.Duration

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

// Validate applies validation rules that are independent of policy source.
func (p *ScalingPolicy) Validate() error {
	if p == nil {
		return nil
	}

	var result *multierror.Error

	if p.Type == "" {
		result = multierror.Append(result, errors.New("policy has not type defined"))
	}

	switch p.OnCheckError {
	case "", ScalingPolicyOnErrorFail, ScalingPolicyOnErrorIgnore:
	default:
		err := fmt.Errorf("invalid value for on_check_error: only %s and %s are allowed",
			ScalingPolicyOnErrorFail, ScalingPolicyOnErrorIgnore)
		result = multierror.Append(result, err)
	}

	if len(p.Checks) == 0 {
		result = multierror.Append(result, fmt.Errorf("empty checks, this policy won't execute any verification or scaling and should have enabled set to false"))
	}

	for _, c := range p.Checks {
		if c.Strategy == nil || c.Strategy.Name == "" {
			result = multierror.Append(result, fmt.Errorf("invalid check %s: missing strategy value", c.Name))
			continue
		}

		if p.Type == ScalingPolicyTypeCluster || p.Type == ScalingPolicyTypeHorizontal {
			if strings.HasPrefix(c.Strategy.Name, "app-sizing") {
				err := fmt.Errorf("invalid strategy in check %s: plugin %s can only be used with Dynamic Application Sizing", c.Name, c.Strategy.Name)
				result = multierror.Append(result, err)
			}
		}

		switch c.OnError {
		case "", ScalingPolicyOnErrorFail, ScalingPolicyOnErrorIgnore:
		default:
			err := fmt.Errorf("invalid value for on_error in check %s: only %s and %s are allowed",
				c.Name, ScalingPolicyOnErrorFail, ScalingPolicyOnErrorIgnore)
			result = multierror.Append(result, err)
		}
	}

	return errHelper.FormattedMultiError(result)
}

// ScalingPolicyCheck is an individual check within a scaling policy.This check
// will be executed in isolation alongside other checks within the policy.
type ScalingPolicyCheck struct {

	// Name is a human readable name for this check and allows operators to
	// create clearly identified policy checks.
	Name string

	// Group is used to group related checks together. Their results will be
	// consolidated into a single action.
	Group string

	// Source is the APM plugin that should be used to perform the query and
	// obtain the metric that will be used to perform a calculation.
	Source string

	// Query is run against the Source in order to receive a metric response.
	Query string

	// QueryWindow is used to define how further back in time to query for
	// metrics.
	QueryWindow time.Duration

	// QueryWindowOffset defines an offset from the current time to apply to
	// the query window.
	QueryWindowOffset time.Duration

	// Strategy is the ScalingPolicyStrategy to use when performing the
	// ScalingPolicyCheck evaluation.
	Strategy *ScalingPolicyStrategy

	// OnError defines how errors are handled by the Autoscaler when running
	// this check. Possible values are "ignore" or "fail". If not set the
	// policy `on_check_error` value will be used.
	//
	// If "ignore" the check is not considered when calculating the final
	// scaling action result.
	// If "fail" the the entire policy evaluation will stop and no action will
	// be taken.
	OnError string
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
	if t == nil || t.Config == nil {
		return false
	}
	_, classOK := t.Config[TargetConfigKeyClass]
	_, dcOK := t.Config[TargetConfigKeyDatacenter]
	return classOK || dcOK
}

type FileDecodeScalingPolicies struct {
	ScalingPolicies []*FileDecodeScalingPolicy `hcl:"scaling,block"`
}

// FileDecodeScalingPolicy is used as an intermediate step when decoding a
// policy from a file. It is needed because the internal Policy object is
// flattened when compared to the literal HCL version. Therefore we cannot
// translate into the internal struct but use this.
type FileDecodeScalingPolicy struct {
	Name    string               `hcl:"name,label"`
	Enabled bool                 `hcl:"enabled,optional"`
	Type    string               `hcl:"type,optional"`
	Min     int64                `hcl:"min,optional"`
	Max     int64                `hcl:"max"`
	Doc     *FileDecodePolicyDoc `hcl:"policy,block"`
}

type FileDecodePolicyDoc struct {
	Cooldown              time.Duration
	CooldownHCL           string `hcl:"cooldown,optional"`
	CooldownOnScaleUp     time.Duration
	CooldownOnScaleUpHCL  string `hcl:"cooldown_on_scale_up,optional"`
	EvaluationInterval    time.Duration
	EvaluationIntervalHCL string                      `hcl:"evaluation_interval,optional"`
	OnCheckError          string                      `hcl:"on_check_error,optional"`
	Checks                []*FileDecodePolicyCheckDoc `hcl:"check,block"`
	Target                *ScalingPolicyTarget        `hcl:"target,block"`
}

type FileDecodePolicyCheckDoc struct {
	Name                 string `hcl:"name,label"`
	Group                string `hcl:"group,optional"`
	Source               string `hcl:"source,optional"`
	Query                string `hcl:"query,optional"`
	QueryWindow          time.Duration
	QueryWindowHCL       string `hcl:"query_window,optional"`
	QueryWindowOffset    time.Duration
	QueryWindowOffsetHCL string                 `hcl:"query_window_offset,optional"`
	OnError              string                 `hcl:"on_error,optional"`
	Strategy             *ScalingPolicyStrategy `hcl:"strategy,block"`
}

// Translate all values from the decoded policy file into our internal policy
// object.
func (fpd *FileDecodeScalingPolicy) Translate() *ScalingPolicy {
	p := &ScalingPolicy{}

	p.Min = fpd.Min
	p.Max = fpd.Max
	p.Enabled = fpd.Enabled
	p.Type = fpd.Type
	p.Cooldown = fpd.Doc.Cooldown
	p.CooldownOnScaleUp = fpd.Doc.CooldownOnScaleUp
	p.EvaluationInterval = fpd.Doc.EvaluationInterval

	p.OnCheckError = fpd.Doc.OnCheckError
	p.Target = fpd.Doc.Target

	fpd.translateChecks(p)

	return p
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
	c.Group = fdc.Group
	c.Source = fdc.Source
	c.Query = fdc.Query
	c.QueryWindow = fdc.QueryWindow
	c.QueryWindowOffset = fdc.QueryWindowOffset
	c.OnError = fdc.OnError
	c.Strategy = fdc.Strategy
}
