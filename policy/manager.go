// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"maps"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type handlerTracker struct {
	cancel     context.CancelFunc
	updates    chan<- *sdk.ScalingPolicy
	cooldownCh chan<- time.Duration // Channel to enforce cooldown on the handler
}

type targetMonitorGetter interface {
	GetTargetReporter(target *sdk.ScalingPolicyTarget) (targetpkg.TargetStatusGetter, error)
}

// Manager tracks policies and controls the lifecycle of each policy handler.
type Manager struct {
	log           hclog.Logger
	policySources map[SourceName]Source
	targetGetter  targetMonitorGetter

	// lock is used to synchronize parallel access to the maps below.
	handlersLock sync.RWMutex

	// handlers are used to track the Go routines monitoring policies.
	handlers map[PolicyID]*handlerTracker

	// metricsInterval is the interval at which the agent is configured to emit
	// metrics. This is used when creating the periodicMetricsReporter.
	metricsInterval time.Duration

	// policyIDsCh is used to report any changes on the list of policy IDs, it is passed
	// down to the MonitorIDs functions.
	policyIDsCh chan IDMessage

	// policyIDsErrCh is used to report errors coming from the MonitorIDs function
	// running on each policy source. It is passed down as part of the MonitorIDsReq
	// along with policyIDsCh.
	policyIDsErrCh chan error
}

// NewManager returns a new Manager.
func NewManager(log hclog.Logger, ps map[SourceName]Source, pm *manager.PluginManager, mInt time.Duration) *Manager {

	return &Manager{
		log:             log.ResetNamed("policy_manager"),
		policySources:   ps,
		targetGetter:    pm,
		handlersLock:    sync.RWMutex{},
		handlers:        make(map[PolicyID]*handlerTracker),
		metricsInterval: mInt,
		policyIDsCh:     make(chan IDMessage, 2),
		policyIDsErrCh:  make(chan error, 2),
	}
}

func (m *Manager) getHandlersNum() int {
	m.handlersLock.Lock()
	defer m.handlersLock.Unlock()

	return len(m.handlers)
}

// Run starts the manager and blocks until the context is canceled.
// Policies that need to be evaluated are sent in the evalCh.
func (m *Manager) Run(ctx context.Context, evalCh chan<- *sdk.ScalingEvaluation) {
	defer m.stopHandlers()
	// Start the metrics reporter.
	go m.periodicMetricsReporter(ctx, m.metricsInterval)

	for {
		// Create a separate context so we can stop the goroutine monitoring the
		// list of policies independently from the parent context.
		monitorCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Start the policy source and listen for changes in the list of policy IDs
		for _, s := range m.policySources {
			m.log.Info("starting policy source", "source", s.Name())
			req := MonitorIDsReq{
				ErrCh:    m.policyIDsErrCh,
				ResultCh: m.policyIDsCh,
			}

			go s.MonitorIDs(monitorCtx, req)
		}

		// monitorPolicies is a blocking function that will only return without errors when
		// the context is cancelled.
		err := m.monitorPolicies(ctx, evalCh)
		if err == nil {
			break
		}

		// Make sure to cancel the monitor's context before starting a new iteration
		cancel()

		// If we reach this point it means an unrecoverable error happened.
		// We should reset any internal state and re-run the policy manager.
		m.log.Debug("re-starting policy manager")

		// Make sure we start the next iteration with an empty map of handlers.
		m.handlersLock.Lock()
		m.handlers = make(map[PolicyID]*handlerTracker)
		m.handlersLock.Unlock()

		// Delay the next iteration of m.Run to avoid re-runs to start too often.
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) monitorPolicies(ctx context.Context, evalCh chan<- *sdk.ScalingEvaluation) error {
	defer m.stopHandlers()

	m.log.Trace(" starting to monitor policies")
	for {
		select {
		case <-ctx.Done():
			m.log.Trace("stopping policy manager")
			return nil

		case err := <-m.policyIDsErrCh:
			m.log.Error("encountered an error monitoring policy IDs", "error", err)
			if isUnrecoverableError(err) {
				return err
			}
			continue

		case message := <-m.policyIDsCh:
			m.log.Trace("received policy IDs listing",
				"num", len(message.IDs), "policy_source", message.Source)

			// Stop the handlers for policies that are no longer present.
			m.handlersLock.Lock()
			maps.DeleteFunc(m.handlers, func(k PolicyID, h *handlerTracker) bool {
				if _, ok := message.IDs[k]; !ok {
					m.log.Trace("stopping handler for removed policy",
						"policy_id", k, "policy_source", message.Source)
					m.stopHandler(k)
					return true
				}

				return false
			})
			m.handlersLock.Unlock()

			// Now send updates to the active policies or create new handlers
			// for any new policies
			for policyID, updated := range message.IDs {

				updatedPolicy := &sdk.ScalingPolicy{}
				var err error

				if !updated {
					continue
				}

				s := m.policySources[message.Source]
				updatedPolicy, err = s.GetLatestVersion(ctx, policyID)
				if err != nil {
					m.log.Error("encountered an error getting the latest version for policy", "policyID", policyID, "error", err)
					if isUnrecoverableError(err) {
						return err
					}
				}

				m.handlersLock.Lock()
				pht := m.handlers[policyID]
				if pht != nil {
					// If the handler already exists, send the updated policy to it.
					m.log.Trace("sending updated policy to existing handler",
						"policy_id", policyID, "policy_source", message.Source)

					pht.updates <- updatedPolicy
					m.handlersLock.Unlock()
					continue
				}

				m.handlersLock.Unlock()

				// If we reach this point it means it is a new policy and we need
				// a new handler for it.
				m.log.Trace("creating new handler",
					"policy_id", policyID, "policy_source", message.Source)

				handlerCtx, handlerCancel := context.WithCancel(ctx)
				upCh := make(chan *sdk.ScalingPolicy, 1)
				cdCh := make(chan time.Duration, 1)

				nht := &handlerTracker{
					updates:    upCh,
					cancel:     handlerCancel,
					cooldownCh: cdCh,
				}

				tg, err := m.targetGetter.GetTargetReporter(updatedPolicy.Target)
				if err != nil {
					m.log.Error("encountered an error getting the target for the policy handler", "policyID", policyID, "error", err)
					if isUnrecoverableError(err) {
						return err
					}
				}

				go RunNewHandler(handlerCtx, HandlerConfig{
					Log:          m.log.Named("policy_handler").With("policy_id", policyID),
					Policy:       updatedPolicy,
					CooldownChan: cdCh,
					UpdatesChan:  upCh,
					ErrChan:      m.policyIDsErrCh,
					TargetGetter: tg,
					EvalsChannel: evalCh,
				})

				// Add the new handler tracker to the manager's internal state.
				m.addHandlerTracker(policyID, nht)
			}
		}
	}
}

func (m *Manager) stopHandlers() {
	m.handlersLock.Lock()
	defer m.handlersLock.Unlock()

	for id := range m.handlers {
		m.stopHandler(id)
		delete(m.handlers, id)
	}
}

func (m *Manager) addHandlerTracker(id PolicyID, nht *handlerTracker) {
	m.handlersLock.Lock()
	m.handlers[id] = nht
	m.handlersLock.Unlock()
}

// stopHandler stops a handler and removes it from the manager's internal
// state storage.
//
// This method is not thread-safe so a RW lock should be acquired before
// calling it.
func (m *Manager) stopHandler(id PolicyID) {
	m.log.Trace("stoping handler for deleted policy ", "policyID", id)

	ht := m.handlers[id]
	close(ht.cooldownCh)
	close(ht.updates)
	ht.cancel()
}

// EnforceCooldown attempts to enforce cooldown on the policy handler
// representing the passed ID.
func (m *Manager) EnforceCooldown(id string, t time.Duration) {
	m.handlersLock.RLock()
	defer m.handlersLock.RUnlock()

	// Attempt to grab the handler and pass the enforcement onto the
	// implementation. Its possible cooldown is requested on a policy which
	// gets removed, its not a problem but log to aid debugging.
	//
	// Thinking(jrasell): when enforcing cooldown, should we calculate the
	// duration based on the remaining time between calling this function and
	// it actually running. Obtaining the lock could cause a delay which may
	// skew the cooldown period, but this is likely very small.
	if handler, ok := m.handlers[id]; ok && handler.cooldownCh != nil {
		handler.cooldownCh <- t
	} else {
		m.log.Debug("attempted to set cooldown on non-existent handler", "policy_id", id)
	}
}

// ReloadSources triggers a reload of all the policy sources.
func (m *Manager) ReloadSources() {
	m.handlersLock.Lock()
	defer m.handlersLock.Unlock()

	// Tell the ID monitors to reload.
	for _, policySource := range m.policySources {
		policySource.ReloadIDsMonitor()
	}
}

// periodicMetricsReporter periodically emits metrics for the policy manager
// which cannot be performed during inline function calls.
func (m *Manager) periodicMetricsReporter(ctx context.Context, interval time.Duration) {

	t := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h := m.getHandlersNum()

			metrics.SetGauge([]string{"policy", "total_num"}, float32(h))
		}
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
