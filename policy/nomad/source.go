// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"errors"
	"fmt"
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
	keySource                  = "source"
	keyQuery                   = "query"
	keyQueryWindow             = "query_window"
	keyQueryWindowOffset       = "query_window_offset"
	keyEvaluationInterval      = "evaluation_interval"
	keyOnCheckError            = "on_check_error"
	keyOnError                 = "on_error"
	keyTarget                  = "target"
	keyChecks                  = "check"
	keyGroup                   = "group"
	keyStrategy                = "strategy"
	keyCooldown                = "cooldown"
	policiesIDsPollingInterval = 500 * time.Millisecond
)

// Ensure NomadSource satisfies the Source interface.
var _ policy.Source = (*Source)(nil)

// Source is an implementation of the Source interface that retrieves
// policies from a Nomad cluster.
type Source struct {
	log             hclog.Logger
	nomad           *api.Client
	nomadLock       sync.RWMutex
	policyProcessor *policy.Processor

	// reloadCh helps coordinate reloading the of the MonitorIDs routine.
	reloadCh chan struct{}
}

// NewNomadSource returns a new Nomad policy source.
func NewNomadSource(log hclog.Logger, nomad *api.Client, policyProcessor *policy.Processor) *Source {
	return &Source{
		log:             log.ResetNamed("nomad_policy_source"),
		nomad:           nomad,
		policyProcessor: policyProcessor,
		reloadCh:        make(chan struct{}),
	}
}

func (s *Source) SetNomadClient(nomad *api.Client) {
	s.nomadLock.Lock()
	defer s.nomadLock.Unlock()
	s.nomad = nomad
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

	q := &api.QueryOptions{}
	ticker := time.NewTicker(policiesIDsPollingInterval)

	for {
		var (
			policies []*api.ScalingPolicyListStub
			meta     *api.QueryMeta
			err      error
		)

		// Perform a blocking query on the Nomad API that returns a stub list
		// of scaling policies. The call is done in a goroutine so we can
		// still listen for the context closing or a reload request.
		//blockingQueryCompleteCh := make(chan struct{})

		// Obtain a handler now so we can release the lock before starting
		// the blocking query.
		s.nomadLock.RLock()
		scaling := s.nomad.Scaling()
		s.nomadLock.RUnlock()

		policies, meta, err = scaling.ListPolicies(q)
		//close(blockingQueryCompleteCh)

		/*
		 */
		// If we get an errors at this point, we should sleep and try again.
		if err != nil {
			policy.HandleSourceError(s.Name(),
				fmt.Errorf("failed to call the Nomad list policies API: %v", err), req.ErrCh)
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
			} else {
				s.log.Info("policy not enabled", "policy_id", p.ID)
			}
		}

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex

		// Send new policy IDs in the channel.
		req.ResultCh <- policy.IDMessage{IDs: policyIDs, Source: s.Name()}

		select {
		case <-ctx.Done():
			s.log.Trace("stopping ID subscription")
			return
		case <-s.reloadCh:
			s.log.Trace("reloading policies")
			continue
		case <-ticker.C:
		}
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
			s.nomadLock.RLock()
			scaling := s.nomad.Scaling()
			s.nomadLock.RUnlock()

			p, meta, err = scaling.GetPolicy(string(req.ID), q)
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
