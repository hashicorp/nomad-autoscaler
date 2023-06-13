// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"errors"
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadAPM "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/nomad/plugin"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// Processor helps process policies and perform common actions on them when
// they are discovered from their source.
type Processor struct {
	defaults  *sdk.ConfigDefaults
	nomadAPMs []string
}

// NewProcessor returns a pointer to a new Processor for use.
func NewProcessor(defaults *sdk.ConfigDefaults, apms []string) *Processor {
	return &Processor{
		defaults:  defaults,
		nomadAPMs: apms,
	}
}

// ApplyPolicyDefaults applies the config defaults to the policy where the
// operator does not supply the parameter. This can be used for both cluster
// and task group policies.
func (pr *Processor) ApplyPolicyDefaults(p *sdk.ScalingPolicy) {
	if p.Cooldown == 0 {
		p.Cooldown = pr.defaults.DefaultCooldown
	}
	if p.EvaluationInterval == 0 {
		p.EvaluationInterval = pr.defaults.DefaultEvaluationInterval
	}

	for i := 0; i < len(p.Checks); i++ {
		c := p.Checks[i]
		if c.QueryWindow == 0 {
			c.QueryWindow = sdk.DefaultQueryWindow
		}
	}
}

// ValidatePolicy performs validation of the policy document returning a list
// of errors found, if any.
func (pr *Processor) ValidatePolicy(p *sdk.ScalingPolicy) error {

	var mErr *multierror.Error

	if p.ID == "" {
		mErr = multierror.Append(mErr, errors.New("policy ID is empty"))
	}
	if p.Min < 0 {
		mErr = multierror.Append(mErr, errors.New("policy Min can't be negative"))
	}
	if p.Max < 0 {
		mErr = multierror.Append(mErr, errors.New("policy Max can't be negative"))
	}
	if p.Min > p.Max {
		mErr = multierror.Append(mErr, errors.New("policy Min must not be greater Max"))
	}

	return mErr.ErrorOrNil()
}

// CanonicalizeCheck sets standardised values on fields.
func (pr *Processor) CanonicalizeCheck(c *sdk.ScalingPolicyCheck, t *sdk.ScalingPolicyTarget) {

	// Operators can omit the check query source which defaults to the Nomad
	// APM.
	if c.Source == "" {
		c.Source = plugins.InternalAPMNomad
	}
	pr.CanonicalizeAPMQuery(c, t)
}

// CanonicalizeAPMQuery takes a short styled Nomad APM check query and creates
// its fully hydrated internal representation. This is required by the Nomad
// APM if it is being used as the source. The function can be called without
// any validation on the check.
func (pr *Processor) CanonicalizeAPMQuery(c *sdk.ScalingPolicyCheck, t *sdk.ScalingPolicyTarget) {

	// Catch nils so this function is safe to call without any prior checks.
	if c == nil || t == nil {
		return
	}

	// If the query source is not a Nomad APM, we do not have any additional
	// work to perform. The APM canonicalization is specific to the Nomad APM.
	if !pr.isNomadAPMQuery(c.Source) {
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
	if t.IsJobTaskGroupTarget() {
		c.Query = fmt.Sprintf("%s_%s/%s/%s",
			nomadAPM.QueryTypeTaskGroup, c.Query, t.Config[sdk.TargetConfigKeyTaskGroup], t.Config[sdk.TargetConfigKeyJob])
		return
	}

	// If the target is a Nomad client node pool, format the query in the
	// expected manner. Once the autoscaler supports more than just class
	// identification of pools this func and logic will need to be updated. For
	// now keep it simple.
	if t.IsNodePoolTarget() {
		c.Query = fmt.Sprintf("%s_%s/%s/class",
			nomadAPM.QueryTypeNode, c.Query, t.Config[sdk.TargetConfigKeyClass])
	}
}

// isNomadAPMQuery helps identify whether the policy query is aligned with a
// configured Nomad APM source.
func (pr *Processor) isNomadAPMQuery(source string) bool {
	for _, name := range pr.nomadAPMs {
		if source == name {
			return true
		}
	}
	return false
}

// isShortQuery detects if a query is in the <type>_<op>_<metric> format which
// is required by the Nomad APM.
func isShortQuery(q string) bool {
	opMetric := strings.SplitN(q, "_", 2)
	hasSlash := strings.Contains(q, "/")
	return len(opMetric) == 2 && !hasSlash
}
