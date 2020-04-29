package nomad

import (
	"context"
	"fmt"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
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
	keyStrategy           = "strategy"
)

const (
	defaultEvaluationInterval = 10 * time.Second
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

// Source is an implementation of the Source interface that retrieves
// policies from a Nomad cluster.
type Source struct {
	log   hclog.Logger
	nomad *api.Client
}

// NewNomadSource returns a new Nomad policy source.
func NewNomadSource(log hclog.Logger, nomad *api.Client) *Source {
	return &Source{
		log:   log.Named("nomad_policy_source"),
		nomad: nomad,
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

			// Iterate all policies in the list.
			for _, p := range policies {
				policyIDs = append(policyIDs, policy.PolicyID(p.ID))
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

			var autoPolicy policy.Policy
			// TODO(jrasell) once we have a better method for surfacing errors to the
			//  user, this error should be presented.
			if autoPolicy, err = parsePolicy(p); err != nil {
				errCh <- fmt.Errorf("failed to parse policy: %v", err)
				return
			}

			resultCh <- autoPolicy
			q.WaitIndex = meta.LastIndex
		}
	}
}
