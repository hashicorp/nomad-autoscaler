package nomad

import (
	"context"
	"fmt"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad/api"
)

// Keys represent the scaling policy document keys and help translate
// the opaque object into a usable autoscaling policy.
const (
	keySource             = "source"
	keyQuery              = "query"
	keyEvaluationInterval = "evaluation_interval"
	keyTarget             = "target"
	keyChecks             = "check"
	keyStrategy           = "strategy"
	keyCooldown           = "cooldown"
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

// SourceConfig holds configuration values for the Nomad source.
type SourceConfig struct {
	DefaultEvaluationInterval time.Duration
	DefaultCooldown           time.Duration
}

func (c *SourceConfig) canonicalize() {
	if c.DefaultEvaluationInterval == 0 {
		c.DefaultEvaluationInterval = policy.DefaultEvaluationInterval
	}
}

// Source is an implementation of the Source interface that retrieves
// policies from a Nomad cluster.
type Source struct {
	log    hclog.Logger
	nomad  *api.Client
	config *SourceConfig
}

// NewNomadSource returns a new Nomad policy source.
func NewNomadSource(log hclog.Logger, nomad *api.Client, config *SourceConfig) *Source {
	config.canonicalize()

	return &Source{
		log:    log.Named("nomad_policy_source"),
		nomad:  nomad,
		config: config,
	}
}

// MonitorIDs retrieves a list of policy IDs from a Nomad cluster and sends it
// in the resultCh channel when change is detected. Errors are sent through the
// errCh channel.
//
// This function blocks until the context is closed.
func (s *Source) MonitorIDs(ctx context.Context, resultCh chan<- []policy.PolicyID, errCh chan<- error) {
	s.log.Debug("starting policy blocking query watcher")

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		select {
		case <-ctx.Done():
			s.log.Trace("stopping ID subscription")
			return
		default:
			// Perform a blocking query on the Nomad API that returns a stub list
			// of scaling policies. If we get an errors at this point, we should
			// sleep and try again.
			//
			// TODO(jrasell) in the future maybe use a better method than sleep.
			policies, meta, err := s.nomad.Scaling().ListPolicies(q)

			// Return immediately if context is closed.
			if ctx.Err() != nil {
				s.log.Trace("stopping ID subscription")
				return
			}

			if err != nil {
				errCh <- fmt.Errorf("failed to call the Nomad list policies API: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			// If the index has not changed, the query returned because the timeout
			// was reached, therefore start the next query loop.
			if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
				continue
			}

			var policyIDs []policy.PolicyID

			// Iterate over all policies in the list and filter out policies
			// that are not enabled.
			for _, p := range policies {
				if p.Enabled {
					policyIDs = append(policyIDs, policy.PolicyID(p.ID))
				}
			}

			// Update the Nomad API wait index to start long polling from the
			// correct point and update our recorded lastChangeIndex so we have the
			// correct point to use during the next API return.
			q.WaitIndex = meta.LastIndex

			// Send new policy IDs in the channel.
			resultCh <- policyIDs
		}
	}
}

// MonitorPolicy monitors a policy and sends it through the resultCh channel
// when a change is detect. Errors are sent through the errCh channel.
//
// This function blocks until the context is closed.
func (s *Source) MonitorPolicy(ctx context.Context, ID policy.PolicyID, resultCh chan<- policy.Policy, errCh chan<- error) {
	log := s.log.With("policy_id", ID)

	// Close channels when done with the monitoring loop.
	defer close(resultCh)
	defer close(errCh)

	log.Trace("starting policy blocking query watcher")

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}
	for {
		select {
		case <-ctx.Done():
			log.Trace("done with policy monitoring")
			return
		default:
			// Perform a blocking query on the Nomad API that returns a stub list
			// of scaling policies. If we get an errors at this point, we should
			// sleep and try again.
			//
			// TODO(jrasell) in the future maybe use a better method than sleep.
			p, meta, err := s.nomad.Scaling().GetPolicy(string(ID), q)

			// Return immediately if context is closed.
			if ctx.Err() != nil {
				log.Trace("done with policy monitoring")
				return
			}

			if err != nil {
				errCh <- fmt.Errorf("failed to get policy: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			// If the index has not changed, the query returned because the timeout
			// was reached, therefore start the next query loop.
			if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
				continue
			}

			if err := validateScalingPolicy(p); err != nil {
				errMsg := "policy validation failed"
				if _, ok := err.(*multierror.Error); ok {
					// Add new error message as first error item.
					err = multierror.Append(fmt.Errorf(errMsg), err)
				} else {
					err = fmt.Errorf("%s: %v", errMsg, err)
				}

				errCh <- err
				continue
			}

			autoPolicy := parsePolicy(p)
			s.canonicalizePolicy(&autoPolicy)

			resultCh <- autoPolicy
			q.WaitIndex = meta.LastIndex
		}
	}
}

// canonicalizePolicy sets standarized values for missing fields.
func (s *Source) canonicalizePolicy(p *policy.Policy) {
	if p == nil {
		return
	}

	// Default EvaluationInterval to the agent's DefaultEvaluationInterval.
	if p.EvaluationInterval == 0 {
		p.EvaluationInterval = s.config.DefaultEvaluationInterval
	}

	// If the operator did not set a cooldown, use the agent's DefaultCooldown.
	if p.Cooldown == 0 {
		p.Cooldown = s.config.DefaultCooldown
	}

	// Set default values for Target.
	if p.Target == nil {
		p.Target = &policy.Target{}
	}

	if p.Target.Name == "" {
		p.Target.Name = plugins.InternalTargetNomad
	}

	if p.Target.Config == nil {
		p.Target.Config = make(map[string]string)
	}

	for _, c := range p.Checks {
		canonicalizeCheck(c, p.Target)
	}
}

func canonicalizeCheck(c *policy.Check, t *policy.Target) {
	// Set default values for Strategy.
	if c.Strategy == nil {
		c.Strategy = &policy.Strategy{}
	}

	if c.Strategy.Config == nil {
		c.Strategy.Config = make(map[string]string)
	}

	// Default source to the Nomad APM.
	if c.Source == "" {
		c.Source = plugins.InternalAPMNomad
	}

	// Expand short Nomad APM query from <op>_<metric> into <op>_<metric>/<group>/<job>.
	// <job> must be the last element so we can parse the query correctly
	// since Nomad allows "/" in job IDs.
	if c.Source == plugins.InternalAPMNomad && isShortQuery(c.Query) {
		c.Query = fmt.Sprintf("%s/%s/%s", c.Query, t.Config["Group"], t.Config["Job"])
	}
}

// isShortQuery detects if a query is in the <op>_<metric> format.
func isShortQuery(q string) bool {
	opMetric := strings.SplitN(q, "_", 2)
	hasSlash := strings.Contains(q, "/")
	return len(opMetric) == 2 && !hasSlash
}
