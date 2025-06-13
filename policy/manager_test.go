// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test/must"
)

// Target mocks
type mockTargetMonitorGetter struct {
	msg *mockStatusGetter
}

func (mtrg *mockTargetMonitorGetter) GetTargetReporter(target *sdk.ScalingPolicyTarget) (targetpkg.TargetStatusGetter, error) {
	return mtrg.msg, nil
}

type mockStatusGetter struct {
	status *sdk.TargetStatus
}

func (msg *mockStatusGetter) Status(config map[string]string) (*sdk.TargetStatus, error) {
	return msg.status, nil
}

// Source mocks
type mockSource struct {
	callsCount    int
	name          SourceName
	latestVersion map[PolicyID]*sdk.ScalingPolicy
}

func (ms *mockSource) GetLatestVersion(ctx context.Context, pID PolicyID) (*sdk.ScalingPolicy, error) {
	ms.callsCount++
	return ms.latestVersion[pID], nil
}

func (ms *mockSource) Name() SourceName {
	return ms.name
}

func (ms *mockSource) MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq)          {}
func (ms *mockSource) ReloadIDsMonitor()                                                    {}
func (ms *mockSource) MonitorPolicy(ctx context.Context, monitorPolicyReq MonitorPolicyReq) {}

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

var ms = &mockSource{
	name: "mock-source",
	latestVersion: map[PolicyID]*sdk.ScalingPolicy{
		policy1.ID: policy1,
		policy2.ID: policy2,
	},
}

var mStatusGetter = &mockStatusGetter{
	status: &sdk.TargetStatus{
		Ready: true,
		Count: 1,
		Meta:  map[string]string{},
	},
}

var nonEmptyHandlerTracker = &handlerTracker{
	updates:    make(chan *sdk.ScalingPolicy, 1),
	cancel:     func() {},
	cooldownCh: make(chan time.Duration, 1),
}

func TestMonitoring(t *testing.T) {

	evalCh := make(chan *sdk.ScalingEvaluation)
	initialProcessTimeout = time.Second

	testedManager := &Manager{
		policyIDsCh:    make(chan IDMessage, 1),
		policyIDsErrCh: make(chan error, 1),
		handlers:       make(map[PolicyID]*handlerTracker),
		log:            hclog.NewNullLogger(),
		lock:           sync.RWMutex{},
		policySources:  map[SourceName]Source{"mock-source": ms},
		targetGetter: &mockTargetMonitorGetter{
			msg: mStatusGetter,
		},
	}

	testCases := []struct {
		name                    string
		inputIDMessage          IDMessage
		initialHandlers         map[PolicyID]*handlerTracker
		expHandlers             int
		expCallsToLatestVersion int
	}{
		{
			name: "add_first_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: true,
				},
				Source: ms.name,
			},
			initialHandlers:         map[PolicyID]*handlerTracker{},
			expHandlers:             1,
			expCallsToLatestVersion: 1,
		},
		{
			name: "add_new_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: true,
				},
				Source: ms.name,
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: nonEmptyHandlerTracker,
			},
			expHandlers:             2,
			expCallsToLatestVersion: 1,
		},
		{
			name: "update_older_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: true,
				},
				Source: ms.name,
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: nonEmptyHandlerTracker,
				policy2.ID: nonEmptyHandlerTracker,
			},
			expHandlers:             2,
			expCallsToLatestVersion: 1,
		},
		{
			name: "no_updates",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
					policy2.ID: false,
				},
				Source: ms.name,
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: nonEmptyHandlerTracker,
				policy2.ID: nonEmptyHandlerTracker,
			},
			expHandlers:             2,
			expCallsToLatestVersion: 0,
		},
		{
			name: "remove_policy",
			inputIDMessage: IDMessage{
				IDs: map[PolicyID]bool{
					policy1.ID: false,
				},
				Source: ms.name,
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: nonEmptyHandlerTracker,
			},
			expHandlers:             1,
			expCallsToLatestVersion: 0,
		},
		{
			name: "remove_all_policies",
			inputIDMessage: IDMessage{
				IDs:    map[PolicyID]bool{},
				Source: ms.name,
			},
			initialHandlers: map[PolicyID]*handlerTracker{
				policy1.ID: nonEmptyHandlerTracker,
				policy2.ID: nonEmptyHandlerTracker,
			},
			expHandlers: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testedManager.handlers = tc.initialHandlers
			ms.callsCount = 0

			go func() {
				err := testedManager.monitorPolicies(context.Background(), evalCh)
				must.NoError(t, err)
			}()

			monitorChn := make(chan struct{}, 1)
			go func() {
				for len(testedManager.handlers) != tc.expHandlers {
				}

				close(monitorChn)
			}()

			testedManager.policyIDsCh <- tc.inputIDMessage

			select {
			case <-time.After(2 * time.Second):
				t.Errorf("time out while waiting for policy to trigger a new handler")
			case <-monitorChn:
			}

			must.Eq(t, len(tc.inputIDMessage.IDs), len(testedManager.handlers))
			must.Eq(t, tc.expCallsToLatestVersion, ms.callsCount)

			for id, _ := range tc.inputIDMessage.IDs {
				ph, ok := testedManager.handlers[id]

				must.True(t, ok)
				must.NotNil(t, ph.cooldownCh)
				must.NotNil(t, ph.cancel)
				must.NotNil(t, ph.updates)
			}
		})
	}
}
