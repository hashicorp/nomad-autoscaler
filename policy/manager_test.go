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
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

var testErrUnrecoverable = errors.New("connection refused")

// Target mocks
type mockTargetMonitorGetter struct {
	count int
	msg   *mockStatusGetter
	err   error
}

func (mtrg *mockTargetMonitorGetter) GetTargetReporter(target *sdk.ScalingPolicyTarget) (targetpkg.TargetStatusGetter, error) {
	mtrg.count++

	return mtrg.msg, mtrg.err
}

type mockStatusGetter struct {
	status *sdk.TargetStatus
	err    error
}

func (msg *mockStatusGetter) Status(config map[string]string) (*sdk.TargetStatus, error) {
	return msg.status, msg.err
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
	Checks:  []*sdk.ScalingPolicyCheck{},
	Target: &sdk.ScalingPolicyTarget{
		Name:   "testTarget",
		Config: map[string]string{},
	},

	EvaluationInterval: 5 * time.Minute,
}

var policy2 = &sdk.ScalingPolicy{
	ID:      "policy2",
	Enabled: true,
	Checks:  []*sdk.ScalingPolicyCheck{},
	Target: &sdk.ScalingPolicyTarget{
		Name:   "testTarget",
		Config: map[string]string{},
	},
	EvaluationInterval: 5 * time.Minute,
}

var mStatusGetter = &mockStatusGetter{
	status: &sdk.TargetStatus{
		Ready: true,
		Count: 1,
		Meta:  map[string]string{},
	},
}

func TestMonitoring(t *testing.T) {
	evalCh := make(chan *sdk.ScalingEvaluation)
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
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
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
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
				},
				policy2.ID: {
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
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
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
				},
				policy2.ID: {
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
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
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
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
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
				},
				policy2.ID: {
					updates:    make(chan *sdk.ScalingPolicy, 1),
					cancel:     func() {},
					cooldownCh: make(chan time.Duration, 1),
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
				targetGetter: &mockTargetMonitorGetter{
					msg: mStatusGetter,
				},
			}

			ctx := context.Background()
			ms.resetCounter()

			go func() {
				err := testedManager.monitorPolicies(ctx, evalCh)
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
				must.NotNil(t, ph.cooldownCh)
				must.NotNil(t, ph.cancel)
				must.NotNil(t, ph.updates)
			}
		})
	}
}

func TestProcessMessageAndUpdateHandlers_SourceError(t *testing.T) {
	evalCh := make(chan *sdk.ScalingEvaluation)

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
			err := testedManager.processMessageAndUpdateHandlers(ctx, evalCh, tc.message)
			must.Eq(t, tc.expectedError, errors.Unwrap(err))
			must.Eq(t, tc.expectedCallCount, ms.callsCount)
		})
	}
}

func TestProcessMessageAndUpdateHandlers_GetTargetReporterError(t *testing.T) {
	evalCh := make(chan *sdk.ScalingEvaluation)

	testCases := []struct {
		name                string
		message             IDMessage
		targetReporterError error
		expectedError       error
		expectedCallCount   int
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
			targetReporterError: errors.New("recoverable error"),
			expectedError:       nil,
			expectedCallCount:   3,
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
					},
					"policy2": {
						ID: "policy2",
					},
					"policy3": {
						ID: "policy3",
					},
				},
				countLock: &sync.Mutex{},
			}

			// Create a mock target reporter
			mStatusGetter := &mockTargetMonitorGetter{
				msg: &mockStatusGetter{},
				err: tc.targetReporterError,
			}

			testedManager := &Manager{
				log:           hclog.NewNullLogger(),
				handlersLock:  sync.RWMutex{},
				policySources: map[SourceName]Source{"mock-source": ms},
				targetGetter:  mStatusGetter,
			}

			ctx := context.Background()
			err := testedManager.processMessageAndUpdateHandlers(ctx, evalCh, tc.message)
			must.Eq(t, tc.expectedError, errors.Unwrap(err))
			must.Eq(t, tc.expectedCallCount, ms.callsCount)
		})
	}
}
