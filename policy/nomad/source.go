// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/blocking"
	"github.com/hashicorp/nomad/api"
)

// Keys represent the scaling policy document keys and help translate
// the opaque object into a usable autoscaling policy.
const (
	keySource             = "source"
	keyQuery              = "query"
	keyQueryWindow        = "query_window"
	keyQueryWindowOffset  = "query_window_offset"
	keyEvaluationInterval = "evaluation_interval"
	keyOnCheckError       = "on_check_error"
	keyOnError            = "on_error"
	keyTarget             = "target"
	keyChecks             = "check"
	keyGroup              = "group"
	keyStrategy           = "strategy"
	keyCooldown           = "cooldown"
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

type modifyIndex = uint64

type policiesGetter interface {
	ListPolicies(q *api.QueryOptions) ([]*api.ScalingPolicyListStub, *api.QueryMeta, error)
	GetPolicy(id string, q *api.QueryOptions) (*api.ScalingPolicy, *api.QueryMeta, error)
}

type nomadPolicyGetter struct {
	nomadLock sync.RWMutex
	nomad     *api.Client
}

func newNomadPolicyGetter(nomad *api.Client) *nomadPolicyGetter {
	return &nomadPolicyGetter{
		nomad: nomad,
	}
}

func (npg *nomadPolicyGetter) ListPolicies(q *api.QueryOptions) ([]*api.ScalingPolicyListStub, *api.QueryMeta, error) {
	npg.nomadLock.RLock()
	scaling := npg.nomad.Scaling()
	npg.nomadLock.RUnlock()

	return scaling.ListPolicies(q)
}

func (npg *nomadPolicyGetter) GetPolicy(id string, q *api.QueryOptions) (*api.ScalingPolicy, *api.QueryMeta, error) {
	npg.nomadLock.RLock()
	scaling := npg.nomad.Scaling()
	npg.nomadLock.RUnlock()

	return scaling.GetPolicy(id, q)
}

// Source is an implementation of the Source interface that retrieves
// policies from a Nomad cluster.
type Source struct {
	log            hclog.Logger
	policiesGetter policiesGetter

	policyProcessor *policy.Processor

	// Map of the current policies used to track changes
	monitoredPolicies map[policy.PolicyID]modifyIndex

	// reloadCh helps coordinate reloading the of the MonitorIDs routine.
	reloadCh chan struct{}

	latestIndex modifyIndex
}

// NewNomadSource returns a new Nomad policy source.
func NewNomadSource(log hclog.Logger, nomad *api.Client, policyProcessor *policy.Processor) *Source {
	return &Source{
		log:               log.ResetNamed("nomad_policy_source"),
		policiesGetter:    newNomadPolicyGetter(nomad),
		policyProcessor:   policyProcessor,
		reloadCh:          make(chan struct{}),
		monitoredPolicies: map[policy.PolicyID]modifyIndex{},
		latestIndex:       1,
	}
}

func (s *Source) SetNomadClient(nomad *api.Client) {
	if pg, ok := s.policiesGetter.(*nomadPolicyGetter); ok {
		pg.nomadLock.Lock()
		pg.nomad = nomad
		pg.nomadLock.Unlock()
	}
}

// Name satisfies the Name function of the policy.Source interface.
func (s *Source) Name() policy.SourceName {
	return policy.SourceNameNomad
}

// ReloadIDsMonitor satisfies the ReloadIDsMonitor function of the
// policy.Source interface.
//
// This currently does nothing but in the future will be useful to allow
// reloading configuration options such as the Nomad client params or the log
// level.
func (s *Source) ReloadIDsMonitor() {
	s.reloadCh <- struct{}{}
}

// MonitorIDs retrieves a list of policy IDs from a Nomad cluster and sends it
// in the resultCh channel when change is detected. Errors are sent through the
// errCh channel.
//
// This function blocks until the context is closed.
func (s *Source) MonitorIDs(ctx context.Context, req policy.MonitorIDsReq) {
	s.log.Debug("starting policy blocking query watcher")

	type results struct {
		policies []*api.ScalingPolicyListStub
		meta     *api.QueryMeta
		err      error
	}
	q := &api.QueryOptions{WaitIndex: s.latestIndex}

	blockingQueryCompleteCh := make(chan results)
	defer close(blockingQueryCompleteCh)

	for {
		// Perform a blocking query on the Nomad API that returns a stub list
		// of scaling policies. The call is done in a goroutine so we can
		// still listen for the context closing or a reload request.
		go func() {

			q.WaitIndex = s.latestIndex
			ps, meta, err := s.policiesGetter.ListPolicies(q)

			// There is no access to the call context, but to avoid writing to
			// a closed channel first check if the context was cancelled.
			select {
			case <-ctx.Done():
				return
			default:
			}

			blockingQueryCompleteCh <- results{
				policies: ps,
				meta:     meta,
				err:      err,
			}
		}()
		r := results{
			policies: []*api.ScalingPolicyListStub{},
			meta:     &api.QueryMeta{},
			err:      nil,
		}

		select {
		case <-ctx.Done():
			s.log.Trace("stopping ID subscription")
			return
		case <-s.reloadCh:
			s.log.Trace("reloading policies")
			continue
		case r = <-blockingQueryCompleteCh:
		}

		// If we get an errors at this point, we should sleep and try again.
		if r.err != nil {
			policy.HandleSourceError(s.Name(), fmt.Errorf("failed to call the Nomad list policies API: %v", r.err), req.ErrCh)
			select {
			case <-ctx.Done():
				s.log.Trace("stopping ID subscription")
				return
			case <-s.reloadCh:
				s.log.Trace("reloading policies")
				continue
			case <-time.After(10 * time.Second):
				continue
			}
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChanged(r.meta.LastIndex, s.latestIndex) {
			continue
		}

		// Let's remove all the dissabled policies from the updates.
		r.policies = slices.DeleteFunc(r.policies, func(p *api.ScalingPolicyListStub) bool {
			return !p.Enabled
		})

		// Now removed the policies that are no longer present  in the
		// updated list of policies meanning they were deleted or disabled.
		maps.DeleteFunc(s.monitoredPolicies, func(policyID policy.PolicyID, _ modifyIndex) bool {
			return !slices.ContainsFunc(r.policies, func(p *api.ScalingPolicyListStub) bool {
				return p.ID == string(policyID)
			})
		})

		// Now let's add all the updated and all the new policies.
		policyUpdates := map[policy.PolicyID]bool{}
		for _, newPolicy := range r.policies {
			policyUpdates[newPolicy.ID] = true

			if oldPolicyModifyIndex, ok := s.monitoredPolicies[newPolicy.ID]; ok {
				policyUpdates[newPolicy.ID] = oldPolicyModifyIndex < newPolicy.ModifyIndex
			}

			s.monitoredPolicies[newPolicy.ID] = newPolicy.ModifyIndex
		}

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		s.latestIndex = r.meta.LastIndex

		// Send new policy IDs in the channel.
		req.ResultCh <- policy.IDMessage{IDs: policyUpdates, Source: s.Name()}
	}
}

// MonitorPolicy monitors a policy and sends it through the resultCh channel
// when a change is detect. Errors are sent through the errCh channel.
//
// This function blocks until the context is closed.
func (s *Source) MonitorPolicy(ctx context.Context, req policy.MonitorPolicyReq) {
	log := s.log.With("policy_id", req.ID)

	// Close channels when done with the monitoring loop.
	defer close(req.ResultCh)
	defer close(req.ErrCh)

	log.Trace("starting policy blocking query watcher")

	q := &api.QueryOptions{WaitIndex: 1}
	for {
		var (
			p    *api.ScalingPolicy
			meta *api.QueryMeta
			err  error
		)

		// Perform a blocking query on the Nomad API that returns a scaling
		// policy. The call is done in a goroutine so we can still listen for
		// the context closing or a reload request.
		blockingQueryCompleteCh := make(chan struct{})
		go func() {
			// Obtain a handler now so we can release the lock before starting
			// the blocking query.

			p, meta, err = s.policiesGetter.GetPolicy(string(req.ID), q)
			close(blockingQueryCompleteCh)
		}()

		select {
		case <-ctx.Done():
			log.Trace("done with policy monitoring")
			return
		case <-req.ReloadCh:
			log.Trace("reloading policy monitor")
			continue
		case <-blockingQueryCompleteCh:
		}

		// Return immediately if context is closed.
		if ctx.Err() != nil {
			log.Trace("done with policy monitoring")
			return
		}

		// If we get an errors at this point, we should sleep and try again.
		if err != nil {
			policy.HandleSourceError(s.Name(), fmt.Errorf("failed to get policy: %w", err), req.ErrCh)
			select {
			case <-ctx.Done():
				log.Trace("done with policy monitoring")
				return
			case <-req.ReloadCh:
				log.Trace("reloading policy monitor")
				continue
			case <-time.After(10 * time.Second):
				continue
			}
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// GH-165: update the wait index. After this point there is a
		// possibility of continuing the loop and without setting the index
		// we will just fast loop indefinitely.
		q.WaitIndex = meta.LastIndex

		if err := validateScalingPolicy(p); err != nil {
			errMsg := "policy validation failed"
			if _, ok := err.(*multierror.Error); ok {
				// Add new error message as first error item.
				err = multierror.Append(errors.New(errMsg), err)
			} else {
				err = fmt.Errorf("%s: %v", errMsg, err)
			}

			policy.HandleSourceError(s.Name(), err, req.ErrCh)
			continue
		}

		autoPolicy := parsePolicy(p)
		s.canonicalizePolicy(&autoPolicy)

		req.ResultCh <- autoPolicy
	}
}

// canonicalizePolicy sets standarized values for missing fields.
func (s *Source) canonicalizePolicy(p *sdk.ScalingPolicy) {
	if p == nil {
		return
	}

	// Assume a policy coming from Nomad without a type is a horizontal policy.
	// TODO: review this assumption.
	if p.Type == "" {
		p.Type = sdk.ScalingPolicyTypeHorizontal
	}

	// Apply the cooldown and evaluation interval defaults if the operator did
	// not pass any values.
	s.policyProcessor.ApplyPolicyDefaults(p)

	// Set default values for Target.
	if p.Target == nil {
		p.Target = &sdk.ScalingPolicyTarget{}
	}

	if p.Target.Config == nil {
		p.Target.Config = make(map[string]string)
	}

	s.canonicalizePolicyByType(p)

	for _, c := range p.Checks {
		s.canonicalizeCheck(c, p.Target)
	}
}

func (s *Source) canonicalizePolicyByType(p *sdk.ScalingPolicy) {
	switch p.Type {
	case "horizontal":
		s.canonicalizeHorizontalPolicy(p)
	case "cluster":
		// Nothing to do for now.
	default:
		s.canonicalizeAdditionalTypes(p)
	}
}

func (s *Source) canonicalizeHorizontalPolicy(p *sdk.ScalingPolicy) {
	if p.Target.Name == "" {
		p.Target.Name = plugins.InternalTargetNomad
	}
}

func (s *Source) canonicalizeCheck(c *sdk.ScalingPolicyCheck, t *sdk.ScalingPolicyTarget) {
	// Set default values for Strategy.
	if c.Strategy == nil {
		c.Strategy = &sdk.ScalingPolicyStrategy{}
	}

	if c.Strategy.Config == nil {
		c.Strategy.Config = make(map[string]string)
	}

	// Canonicalize the check.
	s.policyProcessor.CanonicalizeCheck(c, t)
}

func (s *Source) GetLatestPolicy(ctx context.Context, policyID policy.PolicyID) (*sdk.ScalingPolicy, error) {
	p, _, err := s.policiesGetter.GetPolicy(string(policyID), &api.QueryOptions{})
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		if err != nil {
			return nil, fmt.Errorf("nomad source: unable to get policy")
		}
	}

	if err := validateScalingPolicy(p); err != nil {
		errMsg := "policy validation failed"
		if _, ok := err.(*multierror.Error); ok {
			// Add new error message as first error item.
			err = multierror.Append(errors.New(errMsg), err)
		} else {
			err = fmt.Errorf("%s: %v", errMsg, err)
		}

		return nil, err
	}

	autoPolicy := parsePolicy(p)
	s.canonicalizePolicy(&autoPolicy)

	return &autoPolicy, nil
}
