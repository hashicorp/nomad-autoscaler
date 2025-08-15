// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const DefaultLimiterTimeout = 2 * time.Minute

type handlerTracker struct {
	policyType string
	cancel     context.CancelFunc
	updates    chan<- *sdk.ScalingPolicy
}

type targetGetter interface {
	GetTargetController(target *sdk.ScalingPolicyTarget) (targetpkg.Controller, error)
}

// Manager tracks policies and controls the lifecycle of each policy handler.
type Manager struct {
	log           hclog.Logger
	policySources map[SourceName]Source
	targetGetter  targetGetter

	// lock is used to synchronize parallel access to the maps below.
	handlersLock sync.RWMutex

	// handlers are used to track the Go routines monitoring policies.
	handlers map[SourceName]map[PolicyID]*handlerTracker

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

	// Limiter is in charge of controlling the number of scaling actions happening
	// concurrently.
	*Limiter

	pluginManager dependencyGetter

	// Ent only fields
	evaluateAfter       time.Duration
	historicalAPMGetter HistoricalAPMGetter
}

// NewManager returns a new Manager.
func NewManager(log hclog.Logger, ps map[SourceName]Source, pm *manager.PluginManager, mInt time.Duration, l *Limiter) *Manager {

	return &Manager{
		log:                 log.ResetNamed("policy_manager"),
		policySources:       ps,
		targetGetter:        pm,
		handlersLock:        sync.RWMutex{},
		handlers:            make(map[SourceName]map[PolicyID]*handlerTracker),
		metricsInterval:     mInt,
		policyIDsCh:         make(chan IDMessage, 2),
		policyIDsErrCh:      make(chan error, 2),
		Limiter:             l,
		pluginManager:       pm,
		evaluateAfter:       0,
		historicalAPMGetter: &noopHistoricalAPMGetter{},
	}
}

func (m *Manager) getHandlersNumPerSource(s SourceName) int {
	m.handlersLock.RLock()
	defer m.handlersLock.RUnlock()

	return len(m.handlers[s])
}

func (m *Manager) getHandlersNum() int {
	m.handlersLock.RLock()
	defer m.handlersLock.RUnlock()

	num := 0
	for _, handlers := range m.handlers {
		num += len(handlers)
	}

	return num
}

// Run starts the manager and blocks until the context is canceled.
// Policies that need to be evaluated are sent in the evalCh.
func (m *Manager) Run(ctx context.Context, evalCh chan<- *sdk.ScalingEvaluation) {
	defer m.stopHandlers()
	// Start the metrics reporter.
	go m.periodicMetricsReporter(ctx, m.metricsInterval)

	// Create a separate context so we can stop the goroutine monitoring the
	// list of policies independently from the parent context.
	monitorCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {

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
		err := m.monitorPolicies(ctx)
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
		m.handlers = make(map[SourceName]map[PolicyID]*handlerTracker)
		m.handlersLock.Unlock()

		// Delay the next iteration of m.Run to avoid re-runs to start too often.
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) monitorPolicies(ctx context.Context) error {
	defer m.stopHandlers()

	m.log.Trace(" starting to monitor policies")
	for {
		select {
		case <-ctx.Done():
			m.log.Trace("stopping policy manager")
			close(m.policyIDsCh)
			close(m.policyIDsErrCh)
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
			maps.DeleteFunc(m.handlers[message.Source], func(k PolicyID, h *handlerTracker) bool {
				if _, ok := message.IDs[k]; !ok {
					m.log.Trace("stopping handler for removed policy",
						"policy_id", k, "policy_source", message.Source)
					m.stopHandler(message.Source, k)
					return true
				}

				return false
			})
			m.handlersLock.Unlock()

			// Now send updates to the active policies or create new handlers
			// for any new policies
			err := m.processMessageAndUpdateHandlers(ctx, message)
			if err != nil {
				return err
			}
		}
	}
}

func (m *Manager) processMessageAndUpdateHandlers(ctx context.Context, message IDMessage) error {

	for policyID, updated := range message.IDs {
		updatedPolicy := &sdk.ScalingPolicy{}
		var err error

		if !updated {
			continue
		}

		s := m.policySources[message.Source]
		updatedPolicy, err = s.GetLatestVersion(ctx, policyID)
		if err != nil {
			m.log.Error("encountered an error getting the latest version for policy",
				"policyID", policyID, "error", err)
			if isUnrecoverableError(err) {
				return fmt.Errorf("failed to get latest version for policy %s: %w",
					policyID, err)
			}
			continue
		}

		if updatedPolicy == nil {
			m.log.Error("latest version for policy is nil",
				"policyID", policyID)
			continue
		}

		err = updatedPolicy.Validate()
		if err != nil {
			m.log.Warn("latest version for policy is invalid, old one will keep running",
				"policyID", policyID, "validation_error", err)
			continue
		}

		m.handlersLock.RLock()
		pht := m.handlers[message.Source][policyID]
		m.handlersLock.RUnlock()
		if pht != nil {
			// If the handler already exists, send the updated policy to it.
			m.log.Trace("sending updated policy to existing handler",
				"policy_id", policyID, "policy_source", message.Source)

			pht.updates <- updatedPolicy
			continue
		}

		// If we reach this point it means it is a new policy and we need
		// a new handler for it.
		m.log.Trace("creating new handler",
			"policy_id", policyID, "policy_source", message.Source)

		handlerCtx, handlerCancel := context.WithCancel(ctx)
		upCh := make(chan *sdk.ScalingPolicy, 1)

		nht := &handlerTracker{
			updates:    upCh,
			cancel:     handlerCancel,
			policyType: updatedPolicy.Type,
		}

		m.log.Debug("creating new policy handler",
			"policy_id", updatedPolicy)

		tg, err := m.targetGetter.GetTargetController(updatedPolicy.Target)
		if err != nil {
			m.log.Error("encountered an error getting the target for the policy handler",
				"policyID", policyID, "error", err)
			if isUnrecoverableError(err) {
				return fmt.Errorf("failed to get target for policy %s: %w", policyID, err)
			}
			continue
		}

		ph, err := NewPolicyHandler(HandlerConfig{
			Log: m.log.Named("policy_handler").With("policy_id", policyID,
				"source", message.Source, "target", updatedPolicy.Target.Name,
				"target_config", updatedPolicy.Target.Config),
			Policy:              updatedPolicy,
			UpdatesChan:         upCh,
			ErrChan:             m.policyIDsErrCh,
			TargetController:    tg,
			DependencyGetter:    m.pluginManager,
			Limiter:             m.Limiter,
			HistoricalAPMGetter: m.historicalAPMGetter,
		})
		if err != nil {
			m.log.Error("encountered an error starting the policy handler",
				"policyID", policyID, "error", err)
			if isUnrecoverableError(err) {
				return fmt.Errorf("failed to start the policy handler %s: %w",
					policyID, err)
			}
			continue
		}

		go ph.Run(handlerCtx)

		// Add the new handler tracker to the manager's internal state.
		m.addHandlerTracker(message.Source, policyID, nht)
	}

	return nil
}

func (m *Manager) stopHandlers() {
	m.handlersLock.Lock()
	defer m.handlersLock.Unlock()

	for source, handlers := range m.handlers {
		for id := range handlers {
			m.stopHandler(source, id)
			delete(handlers, id)
		}

		delete(m.handlers, source)
	}
}

// stopHandler stops a handler and removes it from the manager's internal
// state storage.
//
// This method is not thread-safe so a RW lock should be acquired before
// calling it.
func (m *Manager) stopHandler(source SourceName, id PolicyID) {
	m.log.Trace("stopping handler for deleted policy ", "policyID", id)

	ht := m.handlers[source][id]
	ht.cancel()
	close(ht.updates)
}

func (m *Manager) StopHandlersByType(typesToStop ...string) {
	m.handlersLock.Lock()
	defer m.handlersLock.Unlock()

	for source, handlers := range m.handlers {
		for id, tracker := range handlers {
			if slices.Contains(typesToStop, tracker.policyType) {
				m.stopHandler(source, id)
				delete(handlers, id)
			}
		}
	}
}

func (m *Manager) addHandlerTracker(source SourceName, id PolicyID, nht *handlerTracker) {
	m.handlersLock.Lock()
	if m.handlers[source] == nil {
		m.handlers[source] = make(map[PolicyID]*handlerTracker)
	}

	m.handlers[source][id] = nht
	m.handlersLock.Unlock()
}

// ReloadSources triggers a reload of all the policy sources.
func (m *Manager) ReloadSources() {

	// Tell the ID monitors to reload.
	for _, policySource := range m.policySources {
		policySource.ReloadIDsMonitor()
	}
}

func (m *Manager) SetEvaluateAfter(ea time.Duration) {
	m.evaluateAfter = ea
}

func (m *Manager) SetHistoricalAPMGetter(hag HistoricalAPMGetter) {
	m.historicalAPMGetter = hag
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
			metrics.SetGauge([]string{"policy", "queue", "horizontal"}, float32(m.Limiter.QueueSize("horizontal")))
			metrics.SetGauge([]string{"policy", "queue", "cluster"}, float32(m.Limiter.QueueSize("cluster")))
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

type Limiter struct {
	timeout time.Duration
	slots   map[string]chan struct{}
}

var ErrExecutionTimeout = errors.New("timeout while waiting for slot")

func NewLimiter(timeout time.Duration, workersConfig map[string]int) *Limiter {
	return &Limiter{
		timeout: timeout,
		slots: map[string]chan struct{}{
			sdk.ScalingPolicyTypeHorizontal: make(chan struct{}, workersConfig["horizontal"]),
			sdk.ScalingPolicyTypeCluster:    make(chan struct{}, workersConfig["cluster"]),
		},
	}
}

func (l *Limiter) AddQueue(queue string, size int) {
	l.slots[queue] = make(chan struct{}, size)
}

func (l *Limiter) QueueSize(queue string) int {
	return len(l.slots[queue])
}

func (l *Limiter) GetSlot(ctx context.Context, p *sdk.ScalingPolicy) error {
	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-time.After(l.timeout):
		return ErrExecutionTimeout

	case l.slots[p.Type] <- struct{}{}:
	}

	return nil
}

func (l *Limiter) ReleaseSlot(p *sdk.ScalingPolicy) {
	<-l.slots[p.Type]
}
