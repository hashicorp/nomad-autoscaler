// Copyright (c) HashiCorp, Inc.
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

func (mtrg *mockTargetGetter) GetTargetController(target *sdk.ScalingPolicyTarget) (targetpkg.TargetController, error) {
	mtrg.count++

	return mtrg.msg, mtrg.err
}

type mockTargetController struct {
	status *sdk.TargetStatus
	err    error
}

func (msg *mockTargetController) Status(config map[string]string) (*sdk.TargetStatus, error) {
	return msg.status, msg.err
}

func (msg *mockTargetController) Scale(action sdk.ScalingAction, config map[string]string) error {
	return msg.err
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

var mStatusController = &mockTargetController{
	status: &sdk.TargetStatus{
		Ready: true,
		Count: 1,
		Meta:  map[string]string{},
	},
}

func TestMonitoring(t *testing.T) {
	testCases := []struct {
		name                    string
		inputIDMessage          IDMessage
		initialHandlers         map[PolicyID]*handlerTracker
		expCallsToLatestVersion int
	}{
		{
			name: "add_first_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: true,
				},
				Source: "mock-source",
			},
			initialHandlers:         map[PolicyID]*handlerTracker{},
			expCallsToLatestVersion: 1,
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
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
			},
			expCallsToLatestVersion: 1,
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
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
				policy2.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
			},
			expCallsToLatestVersion: 1,
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
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
				policy2.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
			},
			expCallsToLatestVersion: 0,
		},
		{
			name: "remove_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
				},
				Source: "mock-source",
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: {
					updates: make(chan *sdk.ScalingPolicy, 1),
					cancel:  func() {},
				},
			},

			expCallsToLatestVersion: 0,
		},
		{
			name: "remove_all_policies",
			inputIDMessage: IDMessage{
				IDs:    map[PolicyID]bool{},
				Source: "mock-source",
			},
			initialHandlers: map[PolicyID]*handlerTracker{
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := &mockSource{
				countLock: &sync.Mutex{},
				name:      "mock-source",
				latestVersion: map[PolicyID]*sdk.ScalingPolicy{
					policy1.ID: policy1,
					policy2.ID: policy2,
				},
			}

			testedManager := &Manager{
				policyIDsCh:    make(chan IDMessage, 1),
				policyIDsErrCh: make(chan error, 1),
				handlers:       tc.initialHandlers,
				log:            hclog.NewNullLogger(),
				handlersLock:   sync.RWMutex{},
				policySources:  map[SourceName]Source{"mock-source": ms},
				targetGetter: &mockTargetGetter{
					msg: mStatusController,
				},
				pluginManager: &MockDependencyGetter{},
			}

			ctx := context.Background()
			ms.resetCounter()

			go func() {
				err := testedManager.monitorPolicies(ctx)
				must.NoError(t, err)
			}()

			testedManager.policyIDsCh <- tc.inputIDMessage

			// Give the manager time to process the message
			time.Sleep(time.Second)

			must.Eq(t, tc.expCallsToLatestVersion, ms.getCallsCounter())
			must.Eq(t, len(tc.inputIDMessage.IDs), testedManager.getHandlersNum())

			for id := range tc.inputIDMessage.IDs {
				ph, ok := testedManager.handlers[id]

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
						ID: "policy1",
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
						ID: "policy2",
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
						ID: "policy3",
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
