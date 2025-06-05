// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"testing"
	"time"

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
			name:  "sets cooldown from agent",
			input: &sdk.ScalingPolicy{},
			expected: &sdk.ScalingPolicy{
				Type:               sdk.ScalingPolicyTypeHorizontal,
				EvaluationInterval: 10 * time.Second,
				Cooldown:           1 * time.Hour,
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

/*
type mockPolicyGetter struct {
	ps   []*api.ScalingPolicyListStub
	meta *api.QueryMeta
	err  error
}

func (npg *mockPolicyGetter) ListPolicies(q *api.QueryOptions) ([]*api.ScalingPolicyListStub, *api.QueryMeta, error) {
	time.Sleep(time.Second) // Simulate some delay

	return npg.ps, npg.meta, npg.err
}

func (npg *mockPolicyGetter) GetPolicy(id string, q *api.QueryOptions) (*api.ScalingPolicy, *api.QueryMeta, error) {
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
		expectedUpdates           map[policy.PolicyID]policy.PolicyUpdate
		expectedMonitoredPolicies map[policy.PolicyID]modifyIndex
	}{
		{
			name:                     "brand_new_policy",
			policyEnabled:            true,
			policyModifyIndex:        1,
			listModifyIndex:          2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 1,
			},
		},
		{
			name:              "policy_is_updated",
			policyEnabled:     true,
			policyModifyIndex: 2,
			listModifyIndex:   2,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 1,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 2,
			},
		},
		{
			name:              "policy_is_removed",
			policyEnabled:     true,
			policyModifyIndex: 2,
			listModifyIndex:   3,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"existing-policy": 1,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 2,
			},
		},
		{
			name:              "policy_is_disabled",
			policyEnabled:     false,
			policyModifyIndex: 2,
			listModifyIndex:   3,
			initialMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 1,
			},
			expectedMonitoredPolicies: map[policy.PolicyID]modifyIndex{
				"test-policy": 2,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			mpg := &mockPolicyGetter{
				ps: []*api.ScalingPolicyListStub{
					{
						ID:          "test-policy",
						Enabled:     tc.policyEnabled,
						ModifyIndex: tc.policyModifyIndex,
					},
				},
				meta: &api.QueryMeta{
					LastIndex: tc.listModifyIndex,
				},
			}

			testSource := Source{
				log:             hclog.NewNullLogger(),
				policiesGetter:  mpg,
				policyProcessor: pr,
				policies:        map[policy.PolicyID]modifyIndex{},
				latestIndex:     1,
			}

			resultsChannel := make(chan policy.IDMessage, 1)
			errChannel := make(chan error, 1)

			tRequest := policy.MonitorIDsReq{
				ResultCh: resultsChannel,
				ErrCh:    errChannel,
			}

			go testSource.MonitorIDs(context.Background(), tRequest)
			// Optionally, add assertions or checks, here as needed.

			select {
			case mes := <-resultsChannel:

				must.Eq(t, len(testSource.monitoredPolicies, len(mes.IDs)))
					must.Eq(t, tc.expectedUpdates[id], update)
					must.Eq(t, tc.expectedMonitoredPolicies, testSource.policies)
				}

			case <-time.After(2 * time.Second):
				t.Errorf("timed out waiting for results or error")
			}
		})
	}
}
*/
