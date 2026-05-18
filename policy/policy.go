// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	nomadAPM "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/nomad/plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
)

const thresholdWithinBoundsTriggerConfigKey = "within_bounds_trigger"

// Processor helps process policies and perform common actions on them when
// they are discovered from their source.
type Processor struct {
	log       hclog.Logger
	defaults  *ConfigDefaults
	nomadAPMs []string
}

// NewProcessor returns a pointer to a new Processor for use.
func NewProcessor(log hclog.Logger, defaults *ConfigDefaults, apms []string) *Processor {
	return &Processor{
		log:       log.ResetNamed("policy_processor"),
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

	if p.CooldownOnScaleUp == 0 {
		p.CooldownOnScaleUp = p.Cooldown
	}

	if p.EvaluationInterval == 0 {
		p.EvaluationInterval = pr.defaults.DefaultEvaluationInterval
	}

	// we limit the grpc timeout to a 75% of the evaluation interval
	if p.Target != nil {
		grpcTimeout := p.EvaluationInterval * 75 / 100
		p.Target.Config[shared.PluginConfigKeyGRPCTimeout] = grpcTimeout.String()
	}

	for i := 0; i < len(p.Checks); i++ {
		c := p.Checks[i]
		if c.QueryWindow == 0 {
			c.QueryWindow = DefaultQueryWindow
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

	for _, c := range p.Checks {
		if c == nil || !c.QueryInstant || c.Strategy == nil {
			continue
		}

		if c.Strategy.Name != plugins.InternalStrategyThreshold {
			continue
		}

		triggerRaw, ok := c.Strategy.Config[thresholdWithinBoundsTriggerConfigKey]
		if !ok {
			mErr = multierror.Append(mErr,
				fmt.Errorf("check %q: %q must be set to 1 when query_window = %q",
					c.Name, thresholdWithinBoundsTriggerConfigKey, "instant"))
			continue
		}

		triggerValue, err := strconv.Atoi(triggerRaw)
		if err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("check %q: %q must be set to 1 when query_window = %q",
					c.Name, thresholdWithinBoundsTriggerConfigKey, "instant"))
			continue
		}

		if triggerValue != 1 {
			mErr = multierror.Append(mErr,
				fmt.Errorf("check %q: %q must be set to 1 when query_window = %q",
					c.Name, thresholdWithinBoundsTriggerConfigKey, "instant"))
		}
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
	if c == nil {
		return
	}

	// If the query source is not a Nomad APM, we do not have any additional
	// work to perform. The APM canonicalization is specific to the Nomad APM.
	if !pr.isNomadAPMQuery(c.Source) {
		return
	}

	// If the query is already in long form, normalize any old-format
	// node pool queries to the new combined format and return.
	// Operators can write long queries directly if they know the
	// autoscaler internal model.
	if !isShortQuery(c.Query) {
		normalized, err := pr.normalizeNodePoolQuery(c.Query)
		if err != nil {
			pr.log.Warn("failed to normalize node pool query, will fail at evaluation",
				"query", c.Query, "error", err)
			return
		}
		c.Query = normalized
		return
	}

	// Short-query expansion requires a target to determine the query type.
	if t == nil {
		return
	}

	// If the target is a Nomad job task group, format the query in the
	// expected manner.
	if t.IsJobTaskGroupTarget() {
		c.Query = fmt.Sprintf(
			"%s_%s/%s/%s@%s",
			nomadAPM.QueryTypeTaskGroup,
			c.Query,
			t.Config[sdk.TargetConfigKeyTaskGroup],
			t.Config[sdk.TargetConfigKeyJob],
			t.Config[sdk.TargetConfigKeyNamespace],
		)
		return
	}

	// If the target is a Nomad client node pool, format the query in the
	// combined format: node_<op>_<metric>/key1=val1[+key2=val2...]
	if t.IsNodePoolTarget() {
		ids, err := nodepool.NewClusterNodePoolIdentifierList(t.Config)
		if err != nil {
			// Cannot determine pool identifier; leave query as-is.
			return
		}

		c.Query = fmt.Sprintf("%s_%s/%s",
			nomadAPM.QueryTypeNode, c.Query, ids.Encode())
	}
}

// normalizeNodePoolQuery converts old-format node pool queries
// (node_<op>_<metric>/<value>/<key>) into the new combined format
// (node_<op>_<metric>/key=value). Queries already in the new format
// or non-node-pool queries are returned unchanged.
// Returns an error if the old-format key is not recognized.
func (pr *Processor) normalizeNodePoolQuery(q string) (string, error) {
	parts := strings.SplitN(q, "/", 3)

	// Only node pool queries (starting with "node_") need normalization.
	// This prevents taskgroup queries from being incorrectly rewritten.
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "node_") {
		return q, nil
	}

	// If the second part contains "=", it's already in the new combined
	// format (key=val or key=val+key=val). No normalization needed;
	// DecodeCombinedQueryIdentifiers will validate keys at query time.
	if strings.Contains(parts[1], "=") {
		return q, nil
	}

	// Only the old 3-part format needs conversion below.
	if len(parts) != 3 || parts[2] == "" {
		return q, nil
	}

	// Map the old key to the canonical key name.
	key := normalizePoolKey(parts[2])
	if key == "" {
		return "", fmt.Errorf("unrecognized pool identifier key %q in query %q, "+
			"allowed values are: %s, %s, %s",
			parts[2], q, sdk.TargetConfigKeyClass, sdk.TargetConfigKeyDatacenter, sdk.TargetConfigKeyNodePool)
	}

	// URL-encode the value for consistency with the combined format
	// used by ClusterNodePoolIdentifierList.Encode().
	normalized := fmt.Sprintf("%s/%s=%s", parts[0], key, url.QueryEscape(parts[1]))
	pr.log.Debug("normalized legacy node pool query",
		"original", q, "normalized", normalized)
	return normalized, nil
}

// normalizePoolKey maps old-format pool key names to canonical key names.
// Returns empty string for unrecognized keys.
func normalizePoolKey(key string) string {
	switch key {
	case "class":
		return sdk.TargetConfigKeyClass
	case sdk.TargetConfigKeyClass, sdk.TargetConfigKeyDatacenter, sdk.TargetConfigKeyNodePool:
		return key
	default:
		return ""
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
