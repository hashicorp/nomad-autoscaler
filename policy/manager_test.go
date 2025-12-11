// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

var testErrUnrecoverable = errors.New("connection refused")

// MockDependencyGetter is a mock implementation of dependencyGetter for testing.
type MockDependencyGetter struct {
	APMLooker      apm.Looker
	APMLookerErr   error
	StrategyRunner strategy.Runner
	StrategyErr    error
}

func (m *MockDependencyGetter) GetAPMLooker(source string) (apm.Looker, error) {
	return m.APMLooker, m.APMLookerErr
}

func (m *MockDependencyGetter) GetStrategyRunner(name string) (strategy.Runner, error) {
	return m.StrategyRunner, m.StrategyErr
}

// Target mocks
type mockTargetGetter struct {
	count int
	msg   *mockTargetController
	err   error
}

func (mtrg *mockTargetGetter) GetTargetController(target *sdk.ScalingPolicyTarget) (targetpkg.Controller, error) {
	mtrg.count++

	return mtrg.msg, mtrg.err
}

type mockTargetController struct {
	scaleErr    error
	scaleLock   sync.Mutex
	scaleCalled bool
	scaleDelay  time.Duration
	lastAction  sdk.ScalingAction
	status      *sdk.TargetStatus
	statusErr   error
}

func (msg *mockTargetController) getScaleCalled() bool {
	msg.scaleLock.Lock()
	defer msg.scaleLock.Unlock()

	return msg.scaleCalled
}

func (msg *mockTargetController) Status(config map[string]string) (*sdk.TargetStatus, error) {
	return msg.status, msg.statusErr
}

func (msg *mockTargetController) Scale(action sdk.ScalingAction, config map[string]string) error {
	msg.scaleLock.Lock()
	msg.scaleCalled = true
	msg.lastAction = action
	msg.scaleLock.Unlock()

	time.Sleep(msg.scaleDelay) // Simulate some processing delay

	return msg.scaleErr
}

// Source mocks
type mockSource struct {
	countLock     sync.Locker
	callsCount    int
	name          SourceName
	latestVersion map[PolicyID]*sdk.ScalingPolicy
	err           error
}

func (ms *mockSource) resetCounter() {
	ms.countLock.Lock()
	defer ms.countLock.Unlock()

	ms.callsCount = 0
}

func (ms *mockSource) getCallsCounter() int {
	ms.countLock.Lock()
	defer ms.countLock.Unlock()

	return ms.callsCount
}

func (ms *mockSource) GetLatestVersion(ctx context.Context, pID PolicyID) (*sdk.ScalingPolicy, error) {
	ms.countLock.Lock()
	defer ms.countLock.Unlock()

	ms.callsCount++
	return ms.latestVersion[pID], ms.err
}

func (ms *mockSource) Name() SourceName {
	return ms.name
}

func (ms *mockSource) MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq) {}
func (ms *mockSource) ReloadIDsMonitor()                                           {}

var policy1 = &sdk.ScalingPolicy{
	ID:      "policy1",
	Type:    sdk.ScalingPolicyTypeCluster,
	Enabled: true,
	Checks: []*sdk.ScalingPolicyCheck{
		{
			Name: "check1",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "strategy",
			},
		},
		{
			Name: "check2",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "strategy",
			},
		},
	},
	Target: &sdk.ScalingPolicyTarget{
		Name:   "testTarget",
		Config: map[string]string{},
	},

	EvaluationInterval: 5 * time.Minute,
}

var policy2 = &sdk.ScalingPolicy{
	ID:      "policy2",
	Enabled: true,
	Type:    sdk.ScalingPolicyTypeHorizontal,
	Checks: []*sdk.ScalingPolicyCheck{
		{
			Name: "check1",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "strategy",
			},
		},
	},
	Target: &sdk.ScalingPolicyTarget{
		Name:   "testTarget",
		Config: map[string]string{},
	},
	EvaluationInterval: 5 * time.Minute,
}

var policy3 = &sdk.ScalingPolicy{
	ID:      "policy3",
	Type:    sdk.ScalingPolicyTypeHorizontal,
	Enabled: true,
	Checks: []*sdk.ScalingPolicyCheck{
		{
			Name: "check1",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "strategy",
			},
		},
		{
			Name: "check2",
			Strategy: &sdk.ScalingPolicyStrategy{
				Name: "strategy",
			},
		},
	},
	Target: &sdk.ScalingPolicyTarget{
		Name:   "testTarget",
		Config: map[string]string{},
	},

	EvaluationInterval: 5 * time.Minute,
}

var mStatusController = &mockTargetController{
	status: &sdk.TargetStatus{
		Ready: true,
		Count: 1,
		Meta:  map[string]string{},
	},
}

func TestMonitoring(t *testing.T) {
	testCases := []struct {
		name                       string
		inputIDMessage             IDMessage
		initialHandlers            map[SourceName]map[PolicyID]*handlerTracker
		expCallsToLatestVersionMS1 int
		expCallsToLatestVersionMS2 int
	}{
		{
			name: "add_first_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: true,
				},
				Source: "mock-source",
			},
			initialHandlers:            map[SourceName]map[PolicyID]*handlerTracker{},
			expCallsToLatestVersionMS1: 1,
		},
		{
			name: "add_new_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: true,
				},
				Source: "mock-source",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
			expCallsToLatestVersionMS1: 1,
		},
		{
			name: "add_new_policy_from_new_source",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy3.ID: true,
				},
				Source: "mock-source-2",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source-1": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
			expCallsToLatestVersionMS2: 1,
		},

		{
			name: "update_older_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: true,
				},
				Source: "mock-source",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
					policy2.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
				"mock-source-2": {
					policy3.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
			expCallsToLatestVersionMS1: 1,
		},
		{
			name: "no_updates",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: false,
				},
				Source: "mock-source",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
					policy2.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
				"mock-source-2": {
					policy3.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
		},
		{
			name: "remove_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
				},
				Source: "mock-source",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
				"mock-source-2": {
					policy3.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
		},
		{
			name: "remove_all_policies",
			inputIDMessage: IDMessage{
				IDs:    map[PolicyID]bool{},
				Source: "mock-source",
			},
			initialHandlers: map[SourceName]map[PolicyID]*handlerTracker{
				"mock-source": {
					policy1.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
					policy2.ID: {
						updates: make(chan *sdk.ScalingPolicy, 1),
						cancel:  func() {},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms1 := &mockSource{
				countLock: &sync.Mutex{},
				name:      "mock-source",
				latestVersion: map[PolicyID]*sdk.ScalingPolicy{
					policy1.ID: policy1,
					policy2.ID: policy2,
				},
			}

			ms2 := &mockSource{
				countLock: &sync.Mutex{},
				name:      "mock-source-2",
				latestVersion: map[PolicyID]*sdk.ScalingPolicy{
					policy3.ID: policy3,
				},
			}

			testedManager := &Manager{
				policyIDsCh:    make(chan IDMessage, 1),
				policyIDsErrCh: make(chan error, 1),
				handlers:       tc.initialHandlers,
				log:            hclog.NewNullLogger(),
				handlersLock:   sync.RWMutex{},
				policySources: map[SourceName]Source{
					"mock-source":   ms1,
					"mock-source-2": ms2,
				},
				targetGetter: &mockTargetGetter{
					msg: mStatusController,
				},
				pluginManager: &MockDependencyGetter{},
			}

			ctx := context.Background()
			ms1.resetCounter()
			ms2.resetCounter()

			go func() {
				err := testedManager.monitorPolicies(ctx)
				must.NoError(t, err)
			}()

			testedManager.policyIDsCh <- tc.inputIDMessage

			// Give the manager time to process the message
			time.Sleep(time.Second)

			must.Eq(t, tc.expCallsToLatestVersionMS1, ms1.getCallsCounter())
			must.Eq(t, tc.expCallsToLatestVersionMS2, ms2.getCallsCounter())
			must.Eq(t, len(tc.inputIDMessage.IDs), testedManager.getHandlersNumPerSource(tc.inputIDMessage.Source))

			for id := range tc.inputIDMessage.IDs {
				ph, ok := testedManager.handlers[tc.inputIDMessage.Source][id]

				must.True(t, ok)
				must.NotNil(t, ph.cancel)
				must.NotNil(t, ph.updates)
			}
		})
	}
}

func TestProcessMessageAndUpdateHandlers_SourceError(t *testing.T) {
	testCases := []struct {
		name              string
		message           IDMessage
		sourceError       error
		expectedError     error
		expectedCallCount int
	}{
		{
			name: "recoverable source error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			sourceError:       errors.New("recoverable error"),
			expectedError:     nil,
			expectedCallCount: 3,
		},
		{
			name: "unrecoverable source error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			sourceError:       testErrUnrecoverable,
			expectedError:     testErrUnrecoverable,
			expectedCallCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := &mockSource{
				name:          "mock-source",
				err:           tc.sourceError,
				latestVersion: map[PolicyID]*sdk.ScalingPolicy{},
				countLock:     &sync.Mutex{},
			}

			testedManager := &Manager{
				log:           hclog.NewNullLogger(),
				handlersLock:  sync.RWMutex{},
				policySources: map[SourceName]Source{"mock-source": ms},
			}

			ctx := context.Background()
			err := testedManager.processMessageAndUpdateHandlers(ctx, tc.message)
			must.Eq(t, tc.expectedError, errors.Unwrap(err))
			must.Eq(t, tc.expectedCallCount, ms.callsCount)
		})
	}
}

func TestProcessMessageAndUpdateHandlers_GetTargetReporterError(t *testing.T) {
	testCases := []struct {
		name                string
		message             IDMessage
		targetReporterError error
		sourceError         error
		expectedError       error
		expectedCallCount   int
	}{
		{
			name: "recoverable_source_error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			sourceError:       errors.New("recoverable error"),
			expectedError:     nil,
			expectedCallCount: 3,
		},
		{
			name: "unrecoverable_source_error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			targetReporterError: nil,
			sourceError:         testErrUnrecoverable,
			expectedError:       testErrUnrecoverable,
			expectedCallCount:   1,
		},
		{
			name: "recoverable_target_error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			targetReporterError: errors.New("recoverable error"),
			expectedError:       nil,
			expectedCallCount:   3,
		},
		{
			name: "unrecoverable_target_error",
			message: IDMessage{
				IDs: map[PolicyID]bool{
					"policy1": true,
					"policy2": true,
					"policy3": true,
				},
				Source: "mock-source"},
			targetReporterError: testErrUnrecoverable,
			expectedError:       testErrUnrecoverable,
			expectedCallCount:   1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := &mockSource{
				name: "mock-source",
				latestVersion: map[PolicyID]*sdk.ScalingPolicy{
					"policy1": {
						ID:   "policy1",
						Type: sdk.ScalingPolicyTypeCluster,
						Checks: []*sdk.ScalingPolicyCheck{
							{
								Name: "check1",
								Strategy: &sdk.ScalingPolicyStrategy{
									Name: "strategy",
								},
							},
						},
					},
					"policy2": {
						ID:   "policy2",
						Type: sdk.ScalingPolicyTypeHorizontal,
						Checks: []*sdk.ScalingPolicyCheck{
							{
								Name: "check1",
								Strategy: &sdk.ScalingPolicyStrategy{
									Name: "strategy",
								},
							},
						},
					},
					"policy3": {
						ID:   "policy3",
						Type: sdk.ScalingPolicyTypeVerticalCPU,
						Checks: []*sdk.ScalingPolicyCheck{
							{
								Name: "check1",
								Strategy: &sdk.ScalingPolicyStrategy{
									Name: "strategy",
								},
							},
						},
					},
				},
				countLock: &sync.Mutex{},
				err:       tc.sourceError,
			}

			// Create a mock target getter
			mStatusController := &mockTargetGetter{
				msg: &mockTargetController{},
				err: tc.targetReporterError,
			}

			testedManager := &Manager{
				log:           hclog.NewNullLogger(),
				handlersLock:  sync.RWMutex{},
				policySources: map[SourceName]Source{"mock-source": ms},
				targetGetter:  mStatusController,
			}

			ctx := context.Background()
			err := testedManager.processMessageAndUpdateHandlers(ctx, tc.message)
			must.Eq(t, tc.expectedError, errors.Unwrap(err))
			must.Eq(t, tc.expectedCallCount, ms.callsCount)
		})
	}
}
