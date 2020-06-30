package policy

import (
	"context"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
)

// Manager tracks policies and controls the lifecycle of each policy handler.
type Manager struct {
	log           hclog.Logger
	policySource  map[SourceName]Source
	pluginManager *manager.PluginManager

	// lock is used to synchronize parallel access to the maps below.
	lock sync.RWMutex

	// handlers are used to track the Go routines monitoring policies.
	handlers map[PolicyID]*Handler

	// keep is used to mark active policies during reconciliation.
	keep map[PolicyID]bool
}

// NewManager returns a new Manager.
func NewManager(log hclog.Logger, ps map[SourceName]Source, pm *manager.PluginManager) *Manager {
	return &Manager{
		log:           log.ResetNamed("policy_manager"),
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

	policyIDsCh := make(chan IDMessage, 2)
	policyIDsErrCh := make(chan error, 2)

	// Create a separate context so we can stop the goroutine monitoring the
	// list of policies independently from the parent context.
	monitorCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start the policy source and listen for changes in the list of policy IDs
	for _, s := range m.policySource {
		req := MonitorIDsReq{ErrCh: policyIDsErrCh, ResultCh: policyIDsCh}
		go s.MonitorIDs(monitorCtx, req)
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			m.log.Trace("stopping policy manager")
			return

		case err := <-policyIDsErrCh:
			m.log.Error(err.Error())
			if isUnrecoverableError(err) {
				break LOOP
			}
			continue

		case policyIDs := <-policyIDsCh:
			m.log.Trace("received policy IDs listing",
				"num", len(policyIDs.IDs), "policy_source", policyIDs.Source)

			m.lock.Lock()

			// Reset set of policies to keep. We will remove the policies that
			// are not in policyIDs to reconcile our state.
			m.keep = make(map[PolicyID]bool)

			// Iterate over policy IDs and create new handlers if necessary
			for _, policyID := range policyIDs.IDs {

				// Mark policy as must-keep so it doesn't get removed.
				m.keep[policyID] = true

				// Check if we already have a handler for this policy.
				if _, ok := m.handlers[policyID]; ok {
					m.log.Trace("handler already exists",
						"policy_id", policyID, "policy_source", policyIDs.Source)
					continue
				}

				// Create and store a new handler and use its channels to monitor
				// the policy for changes.
				m.log.Trace("creating new handler",
					"policy_id", policyID, "policy_source", policyIDs.Source)

				h := NewHandler(policyID, m.log, m.pluginManager, m.policySource[policyIDs.Source])
				m.handlers[policyID] = h

				go func(ID PolicyID) {
					h.Run(ctx, evalCh)

					// Remove the handler when it stops running.
					m.lock.Lock()
					delete(m.handlers, ID)
					m.lock.Unlock()
				}(policyID)
			}

			// Remove and stop handlers for policies that don't exist anymore
			// for the source which manages them.
			for k, h := range m.handlers {
				if !m.keep[k] && h.policySource.Name() == policyIDs.Source {
					m.stopHandler(h)
				}
			}

			m.lock.Unlock()
		}
	}

	// If we reach this point it means an unrecoverable error happened.
	// We should reset any internal state and re-run the policy manager.
	m.log.Debug("re-starting policy manager")

	// Explicitly stop the handlers and cancel the monitoring context instead
	// of relying on the deferred statements, otherwise the next iteration of
	// m.Run would be executed before they are complete.
	m.stopHandlers()
	m.handlers = make(map[PolicyID]*Handler)
	cancel()

	// Delay the next iteration of m.Run to avoid re-runs to start too often.
	time.Sleep(10 * time.Second)
	go m.Run(ctx, evalCh)
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

// ReloadSources triggers a reload of all the policy sources.
func (m *Manager) ReloadSources() {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Tell the ID monitors to reload.
	for _, policySource := range m.policySource {
		policySource.ReloadIDsMonitor()
	}

	// Instruct each policy handler to reload.
	for _, h := range m.handlers {
		h.reloadCh <- struct{}{}
	}
}

// isUnrecoverableError checks if the input error should be considered
// unrecoverable.
//
// An error is considered unrecoverable if it has the potential to affect the
// policy manager's internal state, and so the policy manager should re-run.
//
// Example of an unrecoverable error: the Nomad server becomes unreachable;
// upon reconnecting, the state of the Nomad cluster could be different from
// what's stored in the policy manager (e.g., a different cluster becomes
// available in the same address).
func isUnrecoverableError(err error) bool {
	unrecoverableErrors := []string{"connection refused", "EOF"}
	for _, e := range unrecoverableErrors {
		if strings.Contains(err.Error(), e) {
			return true
		}
	}
	return false
}
