package policystorage

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestStore_handlePolicyUpdate(t *testing.T) {
	testCases := []struct {
		inputStore   *Store
		inputPolicy  *api.ScalingPolicy
		expectedList map[string]*Policy
		name         string
	}{
		{
			inputStore: &Store{
				State: &Backend{state: map[string]*Policy{"policy-1": {}}},
				log:   hclog.NewNullLogger(),
			},
			inputPolicy: &api.ScalingPolicy{
				ID:        "policy-1",
				Namespace: "default",
				Target:    "/v1/job/job-1/group-1/scale",
				JobID:     "job-1",
				Policy: map[string]interface{}{
					"query":  "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))",
					"source": "prometheus",
					"strategy": []interface{}{
						map[string]interface{}{
							"max":  float64(10),
							"min":  float64(1),
							"name": "target-value",
							"config": []interface{}{
								map[string]interface{}{
									"target": float64(20),
								},
							},
						},
					},
				},
				Enabled: boolToPointer(true),
			},
			expectedList: map[string]*Policy{"policy-1": {
				ID:     "policy-1",
				Source: "prometheus",
				Query:  "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))",
				Target: &Target{
					Name: "local-nomad",
					Config: map[string]string{
						"group":  "group-1",
						"job_id": "job-1",
					},
				},
				Strategy: &Strategy{
					Name: "target-value",
					Min:  1,
					Max:  10,
					Config: map[string]string{
						"target": "20",
					},
				},
			}},
			name: "valid-api-policy: should input with target addition",
		},
	}

	for _, tc := range testCases {
		tc.inputStore.handlePolicyUpdate(tc.inputPolicy)
		assert.Equal(t, tc.expectedList, tc.inputStore.State.List())
	}

}

func TestStore_handlePolicyCleanup(t *testing.T) {
	testCases := []struct {
		inputStore      *Store
		inputPolicyList []*api.ScalingPolicyListStub
		expectedList    map[string]*Policy
		name            string
	}{
		{
			inputStore: &Store{
				State: &Backend{state: map[string]*Policy{"policy-1": {}}},
				log:   hclog.NewNullLogger(),
			},
			inputPolicyList: []*api.ScalingPolicyListStub{{ID: "policy-1"}},
			expectedList:    map[string]*Policy{"policy-1": {}},
			name:            "no-delete: should not delete any policies",
		},
		{
			inputStore: &Store{
				State: &Backend{state: map[string]*Policy{"policy-1": {}, "policy-2": {}}},
				log:   hclog.NewNullLogger(),
			},
			inputPolicyList: []*api.ScalingPolicyListStub{{ID: "policy-1"}},
			expectedList:    map[string]*Policy{"policy-1": {}},
			name:            "single-delete: should delete policy-2",
		},
		{
			inputStore: &Store{
				State: &Backend{state: map[string]*Policy{"policy-1": {}, "policy-2": {}, "policy-3": {}}},
				log:   hclog.NewNullLogger(),
			},
			inputPolicyList: []*api.ScalingPolicyListStub{{ID: "policy-2"}},
			expectedList:    map[string]*Policy{"policy-2": {}},
			name:            "multi-delete: should delete policy-1 and policy-3",
		},
	}

	for _, tc := range testCases {
		tc.inputStore.handlePolicyCleanup(tc.inputPolicyList)
		assert.Equal(t, tc.expectedList, tc.inputStore.State.List())
	}
}

func Test_indexHasChange(t *testing.T) {
	testCases := []struct {
		newValue       uint64
		oldValue       uint64
		expectedReturn bool
	}{
		{
			newValue:       13,
			oldValue:       7,
			expectedReturn: true,
		},
		{
			newValue:       13696,
			oldValue:       13696,
			expectedReturn: false,
		},
		{
			newValue:       7,
			oldValue:       13,
			expectedReturn: false,
		},
	}

	for _, tc := range testCases {
		res := indexHasChange(tc.newValue, tc.oldValue)
		assert.Equal(t, tc.expectedReturn, res)
	}
}

func Test_findMaxFound(t *testing.T) {
	testCases := []struct {
		newValue       uint64
		oldValue       uint64
		expectedReturn uint64
	}{
		{
			newValue:       13,
			oldValue:       7,
			expectedReturn: 13,
		},
		{
			newValue:       13696,
			oldValue:       13696,
			expectedReturn: 13696,
		},
		{
			newValue:       7,
			oldValue:       13,
			expectedReturn: 13,
		},
		{
			newValue:       1,
			oldValue:       0,
			expectedReturn: 1,
		},
	}

	for _, tc := range testCases {
		res := findMaxFound(tc.newValue, tc.oldValue)
		assert.Equal(t, tc.expectedReturn, res)
	}
}

func boolToPointer(b bool) *bool { return &b }
