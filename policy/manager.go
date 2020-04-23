package policy

import (
	"context"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/kr/pretty"
)

// Manager tracks policies and control the lifecycle of each policy handler.
type Manager struct {
	log           hclog.Logger
	policySource  Source
	pluginManager *manager.PluginManager

	// handlers are used to track the Go routines tracking policies.
	handlers map[PolicyID]*Subscription

	// keep is used to mark active policies during reconciliation.
	keep map[PolicyID]bool
}

// NewManager returns a new Manager.
func NewManager(log hclog.Logger, ps Source, pm *manager.PluginManager) *Manager {
	return &Manager{
		log:           log.Named("policy_manager"),
		policySource:  ps,
		pluginManager: pm,
		handlers:      make(map[PolicyID]*Subscription),
		keep:          make(map[PolicyID]bool),
	}
}

// Run starts the manager and blocks until the context is canceled.
//
// Policies that need to be evaluated are sent in the evalCh.
func (m *Manager) Run(ctx context.Context, evalCh chan<- *Evaluation) {

	policyIDsCh := make(chan []PolicyID)

	// Start the policy source and listen for changes in the list of policy IDs
	go m.policySource.Start(ctx, policyIDsCh)

	for {
		select {
		case <-ctx.Done():
			m.log.Trace("done")
			// TODO(luiz): stop handlers
			return
		case policyIDs := <-policyIDsCh:
			m.log.Trace("got new list of IDs")

			for _, policyID := range policyIDs {
				// Check if we have a handler for this policy already.
				// Create one otherwise.
				if _, ok := m.handlers[policyID]; !ok {
					m.log.Trace("creating new handler", "policy_id", policyID)

					// TODO(luiz): maybe buffer these channels to avoid blocking?
					s := &Subscription{
						ID:       policyID,
						PolicyCh: make(chan Policy),
						ErrCh:    make(chan error),
						DoneCh:   make(chan interface{}),
					}

					// Subscribe to changes in the policye
					go m.policySource.Subscribe(s)

					// Start Go routine to generate evaluations.
					go m.handlePolicy(s, evalCh)

					// Store subscription information
					m.handlers[policyID] = s
				}
				// Mark policy as must-keep.
				m.keep[policyID] = true
			}

			// Remove and stop handlers for policies that don't exist anymore
			for k, v := range m.handlers {
				if !m.keep[k] {
					close(v.DoneCh)
					delete(m.handlers, k)
				}
			}
		}
	}
}

// handlePolicy tracks a policy for changes and control when the policy should
// be evaluated.
func (m *Manager) handlePolicy(s *Subscription, evalCh chan<- *Evaluation) {
	log := m.log.ResetNamed("policy_handler").With("policy_id", s.ID)

	log.Trace("start policy handler")

	// Store a local copy of the policy so we can compare it for changes.
	var policy *Policy

	// Start with very long ticker until we receive the right interval.
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.DoneCh:
			log.Trace("done")
			return
		case err := <-s.ErrCh:
			// TODO(luiz) add better error handling
			log.Error(err.Error())
		case p := <-s.PolicyCh:
			log.Trace("policy changed", p, pretty.Sprint(p))

			if policy == nil {
				// We received the proper policy value. We can now create the right ticker
				ticker.Stop()
				ticker = time.NewTicker(p.EvaluationInterval)
			}

			// TODO(luiz): handle policy update

			// Update local copy of the policy
			policy = &p
		case <-ticker.C:
			log.Trace("tick")

			if policy == nil {
				// Ticker is actually not ready yet.
				continue
			}

			// Dispense taret plugin
			targetPlugin, err := m.pluginManager.Dispense(policy.Target.Name, plugins.PluginTypeTarget)
			if err != nil {
				log.Error("failed to dispense plugin", "error", err)
				continue
			}
			targetInst, ok := targetPlugin.Plugin().(targetpkg.Target)
			if !ok {
				log.Error("target plugin is not a taret plugin")
				return
			}

			// Get target status
			status, err := targetInst.Status(policy.Target.Config)
			if err != nil {
				log.Error("failed to get target status", "error", err)
				continue
			}

			if status == nil {
				log.Trace("target doesn't exist anymore", "target", policy.Target.Config)
				return
			}

			// Send evaluation in channel if target is ready
			if status.Ready {
				evalCh <- &Evaluation{
					Policy:       policy,
					TargetStatus: status,
				}
			}
		}
	}
}
