// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

var testErr = errors.New("error!")

type mockStrategyRunner struct {
	t *testing.T

	err error
}

func (m *mockStrategyRunner) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	test.NotNil(m.t, eval)
	test.NotNil(m.t, eval.Metrics)

	return eval, m.err
}

type mockAPMLooker struct {
}

func (m *mockAPMLooker) Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	return sdk.TimestampedMetrics{}, nil
}

func (m *mockAPMLooker) QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	return nil, nil
}

var testPolicy = &sdk.ScalingPolicy{
	ID:  "testPolicy",
	Min: 1,
	Max: 10,
}

var testPolicyOnErrorIgnore = &sdk.ScalingPolicy{
	ID:           "testPolicyOnErrorIgnore",
	Min:          1,
	Max:          10,
	OnCheckError: sdk.ScalingPolicyOnErrorIgnore,
}

var testPolicyOnErrorFail = &sdk.ScalingPolicy{
	ID:           "testPolicyOnErrorFail",
	Min:          1,
	Max:          10,
	OnCheckError: sdk.ScalingPolicyOnErrorFail,
}

func TestCheckHandler_getNewCountFromMetrics(t *testing.T) {
	tests := []struct {
		name           string
		metrics        sdk.TimestampedMetrics
		policy         *sdk.ScalingPolicy
		checkOnError   string
		runErr         error
		expError       error
		expectedAction sdk.ScalingAction
	}{
		{
			name:           "ignore_on_check_error",
			checkOnError:   sdk.ScalingPolicyOnErrorIgnore,
			runErr:         testErr,
			policy:         testPolicy,
			metrics:        sdk.TimestampedMetrics{},
			expError:       nil,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:           "ignore_on_check_error_from_check",
			checkOnError:   "",
			runErr:         testErr,
			policy:         testPolicyOnErrorIgnore,
			metrics:        sdk.TimestampedMetrics{},
			expError:       nil,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:           "fail_on_check_error_from_policy",
			checkOnError:   "",
			runErr:         testErr,
			policy:         testPolicyOnErrorFail,
			metrics:        sdk.TimestampedMetrics{},
			expError:       testErr,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:           "fail_on_check_error_from_check",
			checkOnError:   sdk.ScalingPolicyOnErrorFail,
			runErr:         testErr,
			policy:         testPolicy,
			metrics:        sdk.TimestampedMetrics{},
			expError:       testErr,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:         "success",
			checkOnError: sdk.ScalingPolicyOnErrorFail,
			runErr:       nil,
			policy:       testPolicy,
			metrics:      sdk.TimestampedMetrics{},
			expError:     nil,
			expectedAction: sdk.ScalingAction{
				Meta: map[string]interface{}{
					"nomad_policy_id": testPolicy.ID,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockStrategyRunner{
				t:   t,
				err: tt.runErr,
			}

			ch := &sdk.ScalingPolicyCheck{
				OnError: tt.checkOnError,
				Strategy: &sdk.ScalingPolicyStrategy{
					Name: "strategy",
				},
			}

			runner := newCheckRunner(&CheckRunnerConfig{
				Log:            hclog.NewNullLogger(),
				StrategyRunner: sr,
				Policy:         tt.policy,
			}, ch)

			action, err := runner.GetNewCountFromStrategy(context.Background(), 3, tt.metrics)
			must.Eq(t, tt.expectedAction, action)
			must.Eq(t, tt.expError, errors.Unwrap(err))

		})
	}
}

/* func TestCheckHandler_runAPMQuery(t *testing.T) {
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
