package policy

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestBackend(t *testing.T) {

	// Setup a new backend and ensure the state is empty.
	backend := NewStateBackend()
	assert.Len(t, backend.List(), 0)

	// Write a policy and attempt to read it back out.
	policy1 := generateTestPolicy("job-1", "1")
	backend.Set("job-1", policy1)
	assert.Len(t, backend.List(), 1)
	assert.Equal(t, policy1, backend.List()[policy1.ID])

	// Write a second policy and read it all out.
	policy2 := generateTestPolicy("job-2", "2")
	backend.Set("job-2", policy2)
	assert.Len(t, backend.List(), 2)
	assert.Equal(t, policy2, backend.List()[policy2.ID])

	// Modify the query of policy2 and attempt an overwrite.
	policy2.Query = "scalar(avg(new-better-magic-metric-2))"
	backend.Set("job-2", policy2)
	assert.Len(t, backend.List(), 2)
	assert.Equal(t, policy2, backend.List()[policy2.ID])

	// Write two policies belonging to the same job.
	policy3a := generateTestPolicy("job-3", "3a")
	policy3b := generateTestPolicy("job-3", "3b")
	backend.Set("job-3", policy3a)
	backend.Set("job-3", policy3b)

	// Check the state store has 4 policy entries and 4 job entries.
	assert.Len(t, backend.List(), 4)

	// Delete all the policies for job-3.
	backend.DeletePolicies("job-3")
	assert.Len(t, backend.List(), 2)

	// Write a new policy.
	policy4 := generateTestPolicy("job-4", "4")
	backend.Set("job-4", policy4)

	// Check the state store has 3 policy entries.
	assert.Len(t, backend.List(), 3)

	// Reconcile the state.
	input := []*api.ScalingPolicyListStub{{ID: policy4.ID}}
	backend.ReconcilePolicies(input)
	assert.Len(t, backend.List(), 1)
	assert.Equal(t, policy4, backend.List()[policy4.ID])
}

func generateTestPolicy(resource, id string) *Policy {
	return &Policy{
		ID:     "fake-policy-id-" + id,
		Source: "prometheus-prod",
		Query:  fmt.Sprintf("scalar(avg(magic-metric-%s))", id),
		Target: &Target{
			Config: map[string]string{"job_id": resource},
		},
		Strategy: nil,
	}
}
