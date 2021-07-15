package hash

import (
	"math/rand"
	"testing"
	"time"

	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

func BenchmarkConsistentRing_Update(b *testing.B) {
	benches := []struct {
		numMembers int
		name       string
	}{
		{numMembers: 9, name: "3members"},
		{numMembers: 9, name: "6members"},
		{numMembers: 9, name: "9members"},
		{numMembers: 9, name: "12members"},
	}

	for _, bench := range benches {

		members := generateRingMembers(bench.numMembers)
		ring := NewConsistentRing()

		b.Run(bench.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ring.Update(members)
			}
		})
	}
}

func BenchmarkConsistentRing_GetAssignments(b *testing.B) {
	benches := []struct {
		numMembers  int
		numPolicies int
		name        string
	}{
		{numMembers: 3, numPolicies: 100, name: "3members_100policies"},
		{numMembers: 6, numPolicies: 100, name: "6members_100policies"},
		{numMembers: 9, numPolicies: 100, name: "9members_100policies"},
		{numMembers: 12, numPolicies: 100, name: "12members_100policies"},
		{numMembers: 3, numPolicies: 1000, name: "3members_1000policies"},
		{numMembers: 6, numPolicies: 1000, name: "6members_1000policies"},
		{numMembers: 9, numPolicies: 1000, name: "9members_1000policies"},
		{numMembers: 12, numPolicies: 1000, name: "12members_1000policies"},
	}

	rand.Seed(time.Now().Unix())

	for _, bench := range benches {

		inputMembers := generateRingMembers(bench.numMembers)
		inputPolicies := generateTestPolicies(bench.numPolicies)

		ring := NewConsistentRing()
		ring.Update(inputMembers)

		agentID := inputMembers[rand.Intn(len(inputMembers))]

		b.Run(bench.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ring.GetAssignments(inputPolicies, agentID.String())
			}
		})

	}
}

func generateTestPolicies(num int) []policy.PolicyID {
	out := make([]policy.PolicyID, num)
	for i := 0; i < num; i++ {
		out[i] = policy.PolicyID(uuid.Generate())
	}
	return out
}
