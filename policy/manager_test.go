// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

/* type mockTargetMonitorGetter struct {
	msg *mockStatusGetter
}

func (mtrg *mockTargetMonitorGetter) GetTargetReporter(target *sdk.ScalingPolicyTarget) (targetpkg.TargetStatusGetter, error) {
	return mtrg.msg, nil
}

type mockStatusGetter struct {
	status sdk.TargetStatus
}

func (msg *mockStatusGetter) Status(config map[string]string) (*sdk.TargetStatus, error) {
	return &msg.status, nil
}

type mockSource struct {
	name          SourceName
	latestVersion *sdk.ScalingPolicy
}

func (ms *mockSource) MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq) {}
func (ms *mockSource) GetLatestVersion(ctx context.Context, pID PolicyID) (*sdk.ScalingPolicy, error) {
	return ms.latestVersion, nil
}

func (ms *mockSource) Name() SourceName {
	return ms.name
}

func (ms *mockSource) ReloadIDsMonitor()                                                    {}
func (ms *mockSource) MonitorPolicy(ctx context.Context, monitorPolicyReq MonitorPolicyReq) {}

func TestMonitoring(t *testing.T) {
	tests := []struct {
		name           string
		expectedErr    error
		cancelAfter    time.Duration
		expectEvalSent bool
	}{}

	testedManager := &Manager{
		policyIDsCh:    make(chan IDMessage, 1),
		policyIDsErrCh: make(chan error, 1),
		handlers:       make(map[PolicyID]*handlerTracker),
		log:            hclog.NewNullLogger(),
		lock:           sync.RWMutex{},
	}

	evalCh := make(chan *sdk.ScalingEvaluation, 1)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ms := &mockSource{
				name: "mock-source",
			}

			go func() {
				err := testedManager.monitorPolicies(context.Background(), evalCh)
				fmt.Println(err)
			}()

			testedManager.policyIDsCh <- IDMessage{
				IDs:    map[PolicyID]bool{},
				Source: ms.name,
			}
		})
	}
}
*/
