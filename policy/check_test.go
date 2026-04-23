// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

var testErr = errors.New("error!")

type mockStrategyRunner struct {
	t         *testing.T
	count     int64
	direction sdk.ScaleDirection
	runCalls  int

	err error
}

func (m *mockStrategyRunner) Run(eval *sdk.ScalingCheckEvaluation, count int64) (*sdk.ScalingCheckEvaluation, error) {
	m.runCalls++
	test.NotNil(m.t, eval)
	test.NotNil(m.t, eval.Metrics)
	eval.Action.Count = m.count
	eval.Action.Direction = m.direction

	return eval, m.err
}

type mockAPMLooker struct {
	t          *testing.T
	query      string
	timeRange  sdk.TimeRange
	metrics    sdk.TimestampedMetrics
	err        error
	queryCalls int
}

func (m *mockAPMLooker) Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	m.queryCalls++
	must.StrContains(m.t, m.query, query)
	must.Eq(m.t, m.timeRange, timeRange)

	return m.metrics, m.err
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
			name:           "default_on_check_error_from_policy",
			checkOnError:   "",
			runErr:         testErr,
			policy:         testPolicy,
			metrics:        sdk.TimestampedMetrics{},
			expError:       nil,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:           "unexpected_check_on_error_falls_back_to_policy_fail",
			checkOnError:   "unexpected",
			runErr:         testErr,
			policy:         testPolicyOnErrorFail,
			metrics:        sdk.TimestampedMetrics{},
			expError:       testErr,
			expectedAction: sdk.ScalingAction{},
		},
		{
			name:           "unexpected_check_on_error_falls_back_to_policy_ignore",
			checkOnError:   "unexpected",
			runErr:         testErr,
			policy:         testPolicy,
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

			action, err := runner.getNewCountFromStrategy(context.Background(), 3, tt.metrics)
			must.Eq(t, tt.expectedAction, action)
			must.Eq(t, tt.expError, errors.Unwrap(err))

		})
	}
}

func TestCheckHandler_runAPMQuery(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Date(2020, 1, 1, 0, 10, 0, 0, time.UTC)
	}
	now := nowFunc()
	testQuery := "rate(http_requests_total[5m])"
	testWindow := 10 * time.Minute

	testMetrics := sdk.TimestampedMetrics{
		{Timestamp: now.Add(-time.Minute), Value: 1.0},
		{Timestamp: now, Value: 2.0},
	}

	mockLooker := &mockAPMLooker{
		t:     t,
		query: testQuery,
	}

	tests := []struct {
		name       string
		instant    bool
		metrics    sdk.TimestampedMetrics
		queryError error
		expResult  sdk.TimestampedMetrics
		expErr     error
	}{
		{
			name:      "valid_metrics_returned",
			metrics:   testMetrics,
			expErr:    nil,
			expResult: testMetrics,
		},
		{
			name:      "valid_metrics_returned_instant_query_window",
			instant:   true,
			metrics:   testMetrics,
			expErr:    nil,
			expResult: testMetrics,
		},
		{
			name:       "query_error",
			queryError: testErr,
			expResult:  sdk.TimestampedMetrics{},
			expErr:     testErr,
		},
		{
			name:      "empty metrics",
			expResult: sdk.TimestampedMetrics{},
			expErr:    errNoMetrics,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockLooker.metrics = tc.metrics
			mockLooker.err = tc.queryError
			mockLooker.timeRange = sdk.TimeRange{From: now.Add(-testWindow), To: now}
			if tc.instant {
				mockLooker.timeRange = sdk.TimeRange{From: now, To: now}
			}

			check := &sdk.ScalingPolicyCheck{
				Name:              "check",
				Source:            "mock",
				Query:             testQuery,
				QueryWindow:       testWindow,
				QueryWindowOffset: 0,
				QueryInstant:      tc.instant,
				Strategy: &sdk.ScalingPolicyStrategy{
					Name: "strategy",
				},
			}

			handler := newCheckRunner(&CheckRunnerConfig{
				Log:           hclog.NewNullLogger(),
				MetricsGetter: mockLooker,
				Policy: &sdk.ScalingPolicy{
					ID: "testPolicy",
				},
			}, check)

			result, err := handler.queryMetrics(context.Background(), nil)
			must.Eq(t, tc.expResult, result)
			must.True(t, errors.Is(err, tc.expErr))
		})
	}
}

func TestCheckHandler_runAPMQuery_UsesSharedCacheForIdenticalChecks(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Date(2020, 1, 1, 0, 10, 0, 0, time.UTC)
	}

	now := nowFunc()
	query := "rate(http_requests_total[5m])"
	window := 10 * time.Minute
	metrics := sdk.TimestampedMetrics{
		{Timestamp: now.Add(-time.Minute), Value: 1.0},
		{Timestamp: now, Value: 2.0},
	}

	mockLooker := &mockAPMLooker{
		t:       t,
		query:   query,
		metrics: metrics,
		timeRange: sdk.TimeRange{
			From: now.Add(-window),
			To:   now,
		},
	}

	policy := &sdk.ScalingPolicy{ID: "testPolicy"}
	cache := newQueryMetricsCache()

	runnerA := newCheckRunner(&CheckRunnerConfig{
		Log:           hclog.NewNullLogger(),
		MetricsGetter: mockLooker,
		Policy:        policy,
	}, &sdk.ScalingPolicyCheck{
		Name:              "check-a",
		Source:            "mock",
		Query:             query,
		QueryWindow:       window,
		QueryWindowOffset: 0,
		Strategy: &sdk.ScalingPolicyStrategy{
			Name: "strategy",
		},
	})

	runnerB := newCheckRunner(&CheckRunnerConfig{
		Log:           hclog.NewNullLogger(),
		MetricsGetter: mockLooker,
		Policy:        policy,
	}, &sdk.ScalingPolicyCheck{
		Name:              "check-b",
		Source:            "mock",
		Query:             query,
		QueryWindow:       window,
		QueryWindowOffset: 0,
		Strategy: &sdk.ScalingPolicyStrategy{
			Name: "strategy",
		},
	})

	resultA, errA := runnerA.queryMetrics(context.Background(), cache)
	must.NoError(t, errA)
	resultB, errB := runnerB.queryMetrics(context.Background(), cache)
	must.NoError(t, errB)

	must.Eq(t, metrics, resultA)
	must.Eq(t, metrics, resultB)
	must.Eq(t, 1, mockLooker.queryCalls)
}

func TestCheckHandler_runCheckAndCapCount_OutsideSchedule(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
	}

	sr := &mockStrategyRunner{t: t, count: 10, direction: sdk.ScaleDirectionUp}
	ml := &mockAPMLooker{t: t, query: "query", metrics: sdk.TimestampedMetrics{{Timestamp: nowFunc(), Value: 1}}}

	runner := newCheckRunner(&CheckRunnerConfig{
		Log:            hclog.NewNullLogger(),
		StrategyRunner: sr,
		MetricsGetter:  ml,
		Policy:         testPolicy,
	}, &sdk.ScalingPolicyCheck{
		Name:        "scheduled-check",
		Source:      "mock",
		Query:       "query",
		QueryWindow: time.Minute,
		Schedule: &sdk.ScalingPolicySchedule{
			Start:    "0 9 * * *",
			Duration: "30m",
		},
		Strategy: &sdk.ScalingPolicyStrategy{Name: "strategy"},
	})

	action, err := runner.runCheckAndCapCount(context.Background(), 5, newQueryMetricsCache())
	errMsg := must.Sprint("policy check should not run outside a schedule window")
	must.True(t, errors.Is(err, errCheckOutsideSchedule), errMsg)
	must.Eq(t, sdk.ScalingAction{}, action, errMsg)
	must.Eq(t, 0, ml.queryCalls, errMsg)
	must.Eq(t, 0, sr.runCalls, errMsg)
}

func TestCheckHandler_runCheckAndCapCount_IgnoredStrategyErrorsContinueEvaluation(t *testing.T) {
	nowFunc = func() time.Time {
		return time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
	}

	sr := &mockStrategyRunner{t: t, err: testErr}
	ml := &mockAPMLooker{
		t:       t,
		query:   "query",
		metrics: sdk.TimestampedMetrics{{Timestamp: nowFunc(), Value: 1}},
		timeRange: sdk.TimeRange{
			From: nowFunc().Add(-time.Minute),
			To:   nowFunc(),
		},
	}

	runner := newCheckRunner(&CheckRunnerConfig{
		Log:            hclog.NewNullLogger(),
		StrategyRunner: sr,
		MetricsGetter:  ml,
		Policy:         testPolicy,
	}, &sdk.ScalingPolicyCheck{
		Name:        "error-check",
		Source:      "mock",
		Query:       "query",
		QueryWindow: time.Minute,
		Strategy:    &sdk.ScalingPolicyStrategy{Name: "strategy"},
	})

	action, err := runner.runCheckAndCapCount(context.Background(), 5, newQueryMetricsCache())
	must.NoError(t, err)
	must.Eq(t, sdk.ScaleDirectionNone, action.Direction)

	// Side-effect sanity check: count is capped into policy bounds when the
	// ignored strategy error returns an empty action.
	must.Eq(t, testPolicy.Min, action.Count)
	capped, ok := action.Meta["nomad_autoscaler.count.capped"].(bool)
	must.True(t, ok)
	must.True(t, capped)
	original, ok := action.Meta["nomad_autoscaler.count.original"].(int64)
	must.True(t, ok)
	must.Eq(t, int64(0), original)
	must.Eq(t, 1, ml.queryCalls)
	must.Eq(t, 1, sr.runCalls)
}

func TestCheckHandler_runCheckAndCapCount_FixedValueSkipsAPMQuery(t *testing.T) {
	sr := &mockStrategyRunner{t: t, count: 7, direction: sdk.ScaleDirectionUp}
	ml := &mockAPMLooker{t: t, query: "query", err: testErr}

	runner := newCheckRunner(&CheckRunnerConfig{
		Log:            hclog.NewNullLogger(),
		StrategyRunner: sr,
		MetricsGetter:  ml,
		Policy:         testPolicy,
	}, &sdk.ScalingPolicyCheck{
		Name: "fixed-value-check",
		// source and query are not needed for fixed-value, because the APM query should be skipped entirely.
		QueryWindow: time.Minute,
		Strategy: &sdk.ScalingPolicyStrategy{
			Name: plugins.InternalStrategyFixedValue,
		},
	})

	action, err := runner.runCheckAndCapCount(context.Background(), 5, newQueryMetricsCache())
	must.NoError(t, err)
	test.Eq(t, int64(7), action.Count)
	test.Eq(t, sdk.ScaleDirectionUp, action.Direction)
	test.Eq(t, 0, ml.queryCalls, test.Sprint("should not have made any apm queries"))
	test.Eq(t, 1, sr.runCalls)
}
