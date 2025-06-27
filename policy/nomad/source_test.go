// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/shared"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestSource_canonicalizePolicy(t *testing.T) {
	testCases := []struct {
		name     string
		input    *sdk.ScalingPolicy
		expected *sdk.ScalingPolicy
		cb       func(*api.Config, *policy.ConfigDefaults)
	}{
		{
			name: "full policy",
			input: &sdk.ScalingPolicy{
				ID:                 "string",
				Type:               sdk.ScalingPolicyTypeHorizontal,
				Min:                1,
				Max:                5,
				Enabled:            true,
				EvaluationInterval: time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: "target",
					Config: map[string]string{
						"target_config":  "yes",
						"target_config2": "no",
						"Job":            "job",
						"Group":          "group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:              "check",
						Source:            "source",
						Query:             "query",
						QueryWindow:       5 * time.Minute,
						QueryWindowOffset: 2 * time.Minute,
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy",
							Config: map[string]string{
								"strategy_config1": "yes",
								"strategy_config2": "no",
							},
						},
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				ID:                 "string",
				Type:               sdk.ScalingPolicyTypeHorizontal,
				Min:                1,
				Max:                5,
				Enabled:            true,
				EvaluationInterval: time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: "target",
					Config: map[string]string{
						"target_config":                   "yes",
						"target_config2":                  "no",
						"Job":                             "job",
						"Group":                           "group",
						shared.PluginConfigKeyGRPCTimeout: "750ms",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Name:              "check",
						Source:            "source",
						Query:             "query",
						QueryWindow:       5 * time.Minute,
						QueryWindowOffset: 2 * time.Minute,
						Strategy: &sdk.ScalingPolicyStrategy{
							Name: "strategy",
							Config: map[string]string{
								"strategy_config1": "yes",
								"strategy_config2": "no",
							},
						},
					},
				},
			},
		},
		{
			name:  "set all defaults",
			input: &sdk.ScalingPolicy{},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name:   plugins.InternalTargetNomad,
					Config: map[string]string{},
				},
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "expand query when source is empty",
			input: &sdk.ScalingPolicy{
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{
						"Namespace": "dev",
						"Job":       "job",
						"Group":     "group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Query: "avg_cpu",
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Namespace":                       "dev",
						"Job":                             "job",
						"Group":                           "group",
						shared.PluginConfigKeyGRPCTimeout: "7.5s",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source:      plugins.InternalAPMNomad,
						Query:       "taskgroup_avg_cpu/group/job@dev",
						QueryWindow: policy.DefaultQueryWindow,
						Strategy: &sdk.ScalingPolicyStrategy{
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name: "expand query when source is nomad apm",
			input: &sdk.ScalingPolicy{
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{
						"Namespace": "dev",
						"Job":       "job",
						"Group":     "group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source: plugins.InternalAPMNomad,
						Query:  "avg_cpu",
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Namespace":                       "dev",
						"Job":                             "job",
						"Group":                           "group",
						shared.PluginConfigKeyGRPCTimeout: "7.5s",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source:      plugins.InternalAPMNomad,
						Query:       "taskgroup_avg_cpu/group/job@dev",
						QueryWindow: policy.DefaultQueryWindow,
						Strategy: &sdk.ScalingPolicyStrategy{
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name: "expand query from user-defined values",
			input: &sdk.ScalingPolicy{
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{
						"Namespace": "my_ns",
						"Job":       "my_job",
						"Group":     "my_group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Query: "avg_cpu",
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Namespace":                       "my_ns",
						"Job":                             "my_job",
						"Group":                           "my_group",
						shared.PluginConfigKeyGRPCTimeout: "7.5s",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source:      plugins.InternalAPMNomad,
						Query:       "taskgroup_avg_cpu/my_group/my_job@my_ns",
						QueryWindow: policy.DefaultQueryWindow,
						Strategy: &sdk.ScalingPolicyStrategy{
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name: "don't expand query if not nomad apm",
			input: &sdk.ScalingPolicy{
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source: "not_nomad",
						Query:  "avg_cpu",
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":                             "job",
						"Group":                           "group",
						shared.PluginConfigKeyGRPCTimeout: "7.5s",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source:      "not_nomad",
						Query:       "avg_cpu",
						QueryWindow: policy.DefaultQueryWindow,
						Strategy: &sdk.ScalingPolicyStrategy{
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name: "don't expand query if not in short format",
			input: &sdk.ScalingPolicy{
				Target: &sdk.ScalingPolicyTarget{
					Config: map[string]string{
						"Job":   "job",
						"Group": "group",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Query: "avg_cpu/my_group/my_job",
					},
				},
			},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name: plugins.InternalTargetNomad,
					Config: map[string]string{
						"Job":                             "job",
						"Group":                           "group",
						shared.PluginConfigKeyGRPCTimeout: "7.5s",
					},
				},
				Checks: []*sdk.ScalingPolicyCheck{
					{
						Source:      plugins.InternalAPMNomad,
						Query:       "avg_cpu/my_group/my_job",
						QueryWindow: policy.DefaultQueryWindow,
						Strategy: &sdk.ScalingPolicyStrategy{
							Config: map[string]string{},
						},
					},
				},
			},
		},
		{
			name:  "sets eval interval from agent",
			input: &sdk.ScalingPolicy{},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 5 * time.Second,
				Target: &sdk.ScalingPolicyTarget{
					Name:   plugins.InternalTargetNomad,
					Config: map[string]string{},
				},
			},
			cb: func(_ *api.Config, sourceConfig *policy.ConfigDefaults) {
				sourceConfig.DefaultEvaluationInterval = 5 * time.Second
			},
		},
		{
			name:  "sets cooldown and cooldownonscalingup from agent",
			input: &sdk.ScalingPolicy{},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Cooldown:           1 * time.Hour,
				CooldownOnScaleUp:  1 * time.Hour,
				Target: &sdk.ScalingPolicyTarget{
					Name:   plugins.InternalTargetNomad,
					Config: map[string]string{},
				},
			},
			cb: func(_ *api.Config, sourceConfig *policy.ConfigDefaults) {
				sourceConfig.DefaultCooldown = 1 * time.Hour
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := TestNomadSource(t, tc.cb)
			s.canonicalizePolicy(tc.input)
			must.Eq(t, tc.expected, tc.input)
		})
	}
}

type mockPolicyGetter struct {
	counterLock  sync.Mutex
	callsCounter int
	ps           []*api.ScalingPolicyListStub
	meta         *api.QueryMeta
	err          error
}

func (mpg *mockPolicyGetter) ListPolicies(q *api.QueryOptions) ([]*api.ScalingPolicyListStub, *api.QueryMeta, error) {
	mpg.counterLock.Lock()
	defer mpg.counterLock.Unlock()
	mpg.callsCounter++

	time.Sleep(500 * time.Millisecond) // Simulate some delay

	return mpg.ps, mpg.meta, mpg.err
}

func (mpg *mockPolicyGetter) GetPolicy(id string, q *api.QueryOptions) (*api.ScalingPolicy, *api.QueryMeta, error) {
	return nil, nil, nil
}

func TestMonitoringIDs(t *testing.T) {

	pr := policy.NewProcessor(
		&policy.ConfigDefaults{
			DefaultEvaluationInterval: time.Second,
			DefaultCooldown:           time.Second},
		[]string{},
	)

	testCases := []struct {
		name string
		// Initial setup
		sourcePolicies           []*api.ScalingPolicyListStub
		listModifyIndex          uint64
		initialMonitoredPolicies map[policy.PolicyID]modifyIndex

		// Expected results
		expectedUpdates           map[policy.PolicyID]bool
		expectedMonitoredPolicies map[policy.PolicyID]modifyIndex
	}{
		{
			name: "new_policy_is_added",
			sourcePolicies: []*api.ScalingPolicyListStub{
				{
					ID:          "policy1",
					Enabled:     true,
					ModifyIndex: 1,
				},
			},
			listModifyIndex:          2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{},
			expectedUpdates: map[policy.PolicyID]bool{
				"policy1": true,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy1": 1,
			},
		},
		{
			name: "policy_is_updated",
			sourcePolicies: []*api.ScalingPolicyListStub{
				{
					ID:          "policy1",
					Enabled:     true,
					ModifyIndex: 2,
				},
				{
					ID:          "policy2",
					Enabled:     true,
					ModifyIndex: 1,
				},
			},
			listModifyIndex: 2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy1": 1,
				"policy2": 1,
			},
			expectedUpdates: map[policy.PolicyID]bool{
				"policy1": true,
				"policy2": false,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy1": 2,
				"policy2": 1,
			},
		},
		{
			name: "policy_is_disabled",
			sourcePolicies: []*api.ScalingPolicyListStub{
				{
					ID:          "policy1",
					Enabled:     false,
					ModifyIndex: 2,
				},
				{
					ID:          "policy2",
					Enabled:     true,
					ModifyIndex: 1,
				},
			},
			listModifyIndex: 2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy1": 1,
				"policy2": 1,
			},
			expectedUpdates: map[policy.PolicyID]bool{
				"policy2": false,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy2": 1,
			},
		},
		{
			name:            "policy_is_removed",
			sourcePolicies:  []*api.ScalingPolicyListStub{},
			listModifyIndex: 2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"policy1": 1,
			},
			expectedUpdates:           map[policy.PolicyID]bool{},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{},
		},
		{
			name: "dissabled_is_added",
			sourcePolicies: []*api.ScalingPolicyListStub{
				{
					ID:          "policy1",
					Enabled:     false,
					ModifyIndex: 1,
				},
			},
			listModifyIndex:           2,
			initialMonitoredPolicies:  map[policy.PolicyID]modifyIndex{},
			expectedUpdates:           map[policy.PolicyID]bool{},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			mpg := &mockPolicyGetter{
				callsCounter: 0,
				ps:           tc.sourcePolicies,
				meta: &api.QueryMeta{
					LastIndex: tc.listModifyIndex,
				},
			}

			testSource := Source{
				log:               hclog.NewNullLogger(),
				policiesGetter:    mpg,
				policyProcessor:   pr,
				monitoredPolicies: tc.initialMonitoredPolicies,
				latestIndex:       1,
			}

			resultsChannel := make(chan policy.IDMessage, 1)
			errChannel := make(chan error, 1)

			tRequest := policy.MonitorIDsReq{
				ResultCh: resultsChannel,
				ErrCh:    errChannel,
			}

			go testSource.MonitorIDs(context.Background(), tRequest)

			select {
			case mes := <-resultsChannel:
				must.Eq(t, tc.expectedUpdates, mes.IDs)
				must.Eq(t, tc.expectedMonitoredPolicies, testSource.monitoredPolicies)

			case <-time.After(2 * time.Second):
				t.Errorf("timed out waiting for results or error")
			}
		})
	}
}

func TestMonitoringIDs_NoUpdates(t *testing.T) {
	// This test checks that if the source does not return any updates, the
	// monitorIDs function does not send any messages.
	mpg := &mockPolicyGetter{
		ps: []*api.ScalingPolicyListStub{},
		meta: &api.QueryMeta{
			LastIndex: 1,
		},
	}

	testSource := Source{
		log:            hclog.NewNullLogger(),
		policiesGetter: mpg,
		policyProcessor: policy.NewProcessor(
			&policy.ConfigDefaults{},
			[]string{},
		),
		monitoredPolicies: map[policy.PolicyID]modifyIndex{},
		latestIndex:       1,
	}

	resultsChannel := make(chan policy.IDMessage, 1)
	errChannel := make(chan error, 1)

	tRequest := policy.MonitorIDsReq{
		ResultCh: resultsChannel,
		ErrCh:    errChannel,
	}

	go testSource.MonitorIDs(context.Background(), tRequest)

	select {
	case <-resultsChannel:
		t.Errorf("expected no results, but got a message")
	case <-time.After(3 * time.Second):
		// expected the policy getter to be called at least once

		mpg.counterLock.Lock()
		defer mpg.counterLock.Unlock()

		must.NonZero(t, mpg.callsCounter)
	}
}
