package policystorage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateBackend(t *testing.T) {

	// Setup a new backend and ensure the state is empty.
	backend := newStateBackend()
	assert.Len(t, backend.state, 0)

	// Write a policy and attempt to read it back out.
	policy1 := generateTestPolicy("1")
	backend.set(policy1)
	assert.Len(t, backend.state, 1)
	assert.Equal(t, policy1, backend.List()[policy1.ID])

	// Write a second policy and read it all out.
	policy2 := generateTestPolicy("2")
	backend.set(policy2)
	assert.Len(t, backend.state, 2)
	assert.Equal(t, policy2, backend.List()[policy2.ID])

	// Delete policy1.
	backend.delete(policy1.ID)
	assert.Nil(t, backend.List()[policy1.ID])

	// Modify the query of policy2 and attempt an overwrite.
	policy2.Query = "scalar(avg(new-better-magic-metric-2))"
	backend.set(policy2)
	assert.Len(t, backend.state, 1)
	assert.Equal(t, policy2, backend.List()[policy2.ID])
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
