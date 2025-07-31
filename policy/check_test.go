// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

/*
import (
	"context"
	"errors"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRunner struct {
	mock.Mock
}

func (m *mockRunner) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	args := m.Called(eval, count)
	return args.Get(0).(*sdk.ScalingCheckEvaluation), args.Error(1)
}

type mockLooker struct {
	mock.Mock
}

func (m *mockLooker) Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	args := m.Called(query, timeRange)
	return args.Get(0).(sdk.TimestampedMetrics), args.Error(1)
}

func (m *mockLooker) QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	return nil, nil
}

func TestCheckHandler_getNewCountFromMetrics(t *testing.T) {
	tests := []struct {
		name        string
		checkErr    string
		policyErr   string
		runErr      error
		expectError bool
		expectNone  bool
	}{
		{
			name:        "ignore on check error",
			checkErr:    sdk.ScalingPolicyOnErrorIgnore,
			runErr:      errors.New("mock failure"),
			expectError: false,
			expectNone:  true,
		},
		{
			name:        "fail on check error",
			checkErr:    sdk.ScalingPolicyOnErrorFail,
			runErr:      errors.New("mock failure"),
			expectError: true,
		},
		{
			name:        "fallback to policy OnCheckError=fail",
			checkErr:    "",
			policyErr:   sdk.ScalingPolicyOnErrorFail,
			runErr:      errors.New("mock failure"),
			expectError: true,
		},
		{
			name:        "successful run",
			checkErr:    "",
			runErr:      nil,
			expectError: false,
			expectNone:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := new(mockRunner)

			check := &sdk.ScalingPolicyCheck{
				Name:     "check",
				OnError:  tt.checkErr,
				Strategy: &sdk.ScalingPolicyStrategy{Name: "strategy"},
			}
			policy := &sdk.ScalingPolicy{
				ID:           "policy-1",
				Min:          1,
				Max:          10,
				OnCheckError: tt.policyErr,
			}

			runner := newCheckRunner(&CheckRunnerConfig{
				Log:            hclog.NewNullLogger(),
				StrategyRunner: mockRunner,
				Policy:         policy,
			}, check)

			if tt.runErr == nil {
				// return a successful ScalingAction
				resp := &sdk.ScalingCheckEvaluation{
					Action: &sdk.ScalingAction{
						Count:     5,
						Direction: sdk.ScaleDirectionUp,
					},
				}
				mockRunner.On("Run", mock.Anything, int64(3)).Return(resp, nil)
			} else {
				mockRunner.On("Run", mock.Anything, int64(3)).Return(nil, tt.runErr)
			}

			action, err := runner.GetNewCountUsingMetrics(context.Background(), 3, nil)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectNone {
					assert.Equal(t, sdk.ScaleDirectionNone, action.Direction)
				}
			}
		})
	}
}

func TestCheckHandler_runAPMQuery(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		metrics    sdk.TimestampedMetrics
		queryError error
		expectErr  bool
	}{
		{
			name: "valid metrics returned",
			metrics: sdk.TimestampedMetrics{
				{Timestamp: now.Add(-time.Minute), Value: 1.0},
				{Timestamp: now, Value: 2.0},
			},
			expectErr: false,
		},
		{
			name:       "query error",
			queryError: errors.New("no data"),
			expectErr:  true,
		},
		{
			name:      "empty metrics",
			metrics:   sdk.TimestampedMetrics{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLooker := new(mockLooker)

			check := &sdk.ScalingPolicyCheck{
				Name:              "check",
				Source:            "mock",
				Query:             "rate(http_requests_total[5m])",
				QueryWindow:       10 * time.Minute,
				QueryWindowOffset: 0,
				Strategy:          &sdk.ScalingPolicyStrategy{Name: "s"},
			}
			policy := &sdk.ScalingPolicy{ID: "xyz"}

			// Match any reasonable time range
			mockLooker.On("Query", check.Query, mock.Anything).Return(tt.metrics, tt.queryError)

			handler := newCheckRunner(&CheckRunnerConfig{
				Log:           hclog.NewNullLogger(),
				MetricsGetter: mockLooker,
				Policy:        policy,
			}, check)

			result, err := handler.RunAPMQuery(context.Background())

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.metrics, result)
			}
		})
	}
}
*/
