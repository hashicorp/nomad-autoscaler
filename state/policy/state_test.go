package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackend(t *testing.T) {

	// Setup a new backend and ensure the state is empty.
	backend := NewStateBackend()
	assert.Len(t, backend.state, 0)

	// Write a policy and attempt to read it back out.
	policy1 := generateTestPolicy("1")
	backend.Set("job-1", policy1)
	assert.Len(t, backend.state, 1)
	assert.Equal(t, policy1, backend.List()[policy1.ID])

	// Write a second policy and read it all out.
	policy2 := generateTestPolicy("2")
	backend.Set("job-2", policy2)
	assert.Len(t, backend.state, 2)
	assert.Equal(t, policy2, backend.List()[policy2.ID])

	// Delete policy1.
	backend.DeletePolicy("job-1", policy1.ID)
	assert.Nil(t, backend.List()[policy1.ID])

	// Modify the query of policy2 and attempt an overwrite.
	policy2.Query = "scalar(avg(new-better-magic-metric-2))"
	backend.Set("job-2", policy2)
	assert.Len(t, backend.state, 1)
	assert.Equal(t, policy2, backend.List()[policy2.ID])

	// Write two policies belonging to the same job.
	policy3a := generateTestPolicy("3a")
	policy3b := generateTestPolicy("3b")
	backend.Set("job-3", policy3a)
	backend.Set("job-3", policy3b)

	// Check the state store has 3 policy entries and two job entries.
	assert.Len(t, backend.state, 3)
	assert.Len(t, backend.resourceIDs, 2)

	// Delete all the policies for job-3.
	backend.DeletePolicies("job-3")
	assert.Len(t, backend.state, 1)
	assert.Len(t, backend.resourceIDs, 1)

	// Add a Nomad node client class scaling policy.
	policy4 := generateTestPolicy("4")
	backend.Set("high-compute", policy4)
	assert.Len(t, backend.state, 2)
	assert.Len(t, backend.resourceIDs, 2)
	assert.Equal(t, policy4, backend.List()[policy4.ID])
}

func generateTestPolicy(id string) *Policy {
	return &Policy{
		ID:       "fake-policy-id-" + id,
		Source:   "prometheus-prod",
		Query:    fmt.Sprintf("scalar(avg(magic-metric-%s))", id),
		Target:   nil,
		Strategy: nil,
	}
}
