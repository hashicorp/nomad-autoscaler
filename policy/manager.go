package policy

import (
	"context"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
)

// Manager tracks policies and controls the lifecycle of each policy handler.
type Manager struct {
	log           hclog.Logger
	policySource  Source
	pluginManager *manager.PluginManager

	// lock is used to synchronize parallel access to the maps below.
	lock sync.RWMutex

	// handlers are used to track the Go routines monitoring policies.
	handlers map[PolicyID]*Handler

	// keep is used to mark active policies during reconciliation.
	keep map[PolicyID]bool
}

// NewManager returns a new Manager.
func NewManager(log hclog.Logger, ps Source, pm *manager.PluginManager) *Manager {
	return &Manager{
		log:           log.Named("policy_manager"),
		policySource:  ps,
		pluginManager: pm,
		handlers:      make(map[PolicyID]*Handler),
		keep:          make(map[PolicyID]bool),
	}
}

// Run starts the manager and blocks until the context is canceled.
// Policies that need to be evaluated are sent in the evalCh.
func (m *Manager) Run(ctx context.Context, evalCh chan<- *Evaluation) {
	defer m.stopHandlers()

	policyIDsCh := make(chan []PolicyID)
	policyIDsErrCh := make(chan error)

	// Start the policy source and listen for changes in the list of policy IDs
	go m.policySource.MonitorIDs(ctx, policyIDsCh, policyIDsErrCh)

	for {
		select {
		case <-ctx.Done():
			m.log.Trace("stopping policy manager")
			return
		case policyIDs := <-policyIDsCh:
			m.log.Trace(fmt.Sprintf("detected %d policies", len(policyIDs)))

			m.lock.Lock()

			// Reset set of policies to keep. We will remove the policies that
			// are not in policyIDs to reconcile our state.
			m.keep = make(map[PolicyID]bool)

			// Iterate over policy IDs and create new handlers if necessary
			for _, policyID := range policyIDs {

				// Mark policy as must-keep so it doesn't get removed.
				m.keep[policyID] = true

				// Check if we already have a handler for this policy.
				if _, ok := m.handlers[policyID]; ok {
					m.log.Trace("handler already exists")
					continue
				}

				// Create and store a new handler and use its channels to monitor
				// the policy for changes.
				m.log.Trace("creating new handler", "policy_id", policyID)

				h := NewHandler(
					policyID,
					m.log.ResetNamed(""),
					m.pluginManager,
					m.policySource)
				m.handlers[policyID] = h

				go func(ID PolicyID) {
					h.Run(ctx, evalCh)

					m.lock.Lock()
					delete(m.handlers, ID)
					m.lock.Unlock()
				}(policyID)
			}

			// Remove and stop handlers for policies that don't exist anymore.
			for k, h := range m.handlers {
				if !m.keep[k] {
					m.stopHandler(h)
				}
			}

			m.lock.Unlock()
		}
	}
}

func (m *Manager) stopHandlers() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, h := range m.handlers {
		m.stopHandler(h)
	}
}

// stopHandler stops a handler and removes it from the manager's internal
// state storage.
//
// This method is not thread-safe so a RW lock should be acquired before
// calling it.
func (m *Manager) stopHandler(h *Handler) {
	if h == nil {
		return
	}

	h.Stop()
	delete(m.handlers, h.policyID)
}

// EnforceCooldown attempts to enforce cooldown on the policy handler
// representing the passed ID.
func (m *Manager) EnforceCooldown(id string, t time.Duration) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// Attempt to grab the handler and pass the enforcement onto the
	// implementation. Its possible cooldown is requested on a policy which
	// gets removed, its not a problem but log to aid debugging.
	//
	// Thinking(jrasell): when enforcing cooldown, should we calculate the
	// duration based on the remaining time between calling this function and
	// it actually running. Obtaining the lock could cause a delay which may
	// skew the cooldown period, but this is likely very small.
	if handler, ok := m.handlers[PolicyID(id)]; ok && handler.cooldownCh != nil {
		handler.cooldownCh <- t
	} else {
		m.log.Debug("attempted to set cooldown on non-existent handler", "policy_id", id)
	}
}
