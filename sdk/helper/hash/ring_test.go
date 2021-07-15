package hash

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConsistentRing_Update(t *testing.T) {
	testCases := []struct {
		inputConsistentRing *ConsistentRing
		inputUpdateMembers  []RingMember
		name                string
	}{
		{
			inputConsistentRing: NewConsistentRing(),
			inputUpdateMembers:  generateRingMembers(3),
			name:                "empty ring",
		},
		{
			inputConsistentRing: &ConsistentRing{
				hasher:     NewDefaultHasher(),
				memberKeys: []uint32{989091847, 3928713472, 1421016843},
				ring: map[uint32]RingMember{
					989091847:  NewRingMember("04a303cf-9cd2-8faf-02e3-2ad3a387cd55"),
					3928713472: NewRingMember("8a52ccc4-6bc7-e73f-5624-e5da4a935632"),
					1421016843: NewRingMember("68943bff-6b83-476e-a613-d4ef77bf500b"),
				},
			},
			inputUpdateMembers: []RingMember{
				NewRingMember("68943bff-6b83-476e-a613-d4ef77bf500b"),
				NewRingMember("04a303cf-9cd2-8faf-02e3-2ad3a387cd55"),
			},
			name: "populated ring",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputConsistentRing.Update(tc.inputUpdateMembers)
			assert.Len(t, tc.inputConsistentRing.memberKeys, len(tc.inputUpdateMembers), tc.name)
			assert.Len(t, tc.inputConsistentRing.ring, len(tc.inputUpdateMembers), tc.name)

			for _, member := range tc.inputUpdateMembers {
				h := tc.inputConsistentRing.hasher.Hash32(member.Bytes())
				mem, ok := tc.inputConsistentRing.ring[h]
				assert.True(t, ok, tc.name)
				assert.Equal(t, member.String(), mem.String(), tc.name)
			}
		})
	}
}

func TestConsistentRing_GetAssignments(t *testing.T) {
	testCases := []struct {
		inputConsistentRing    *ConsistentRing
		inputPolicies          []policy.PolicyID
		inputAgentID           string
		expectedOutputPolicies []policy.PolicyID
		name                   string
	}{
		{
			inputConsistentRing: &ConsistentRing{
				hasher:     NewDefaultHasher(),
				memberKeys: []uint32{989091847, 3928713472, 1421016843},
				ring: map[uint32]RingMember{
					989091847:  NewRingMember("04a303cf-9cd2-8faf-02e3-2ad3a387cd55"),
					3928713472: NewRingMember("8a52ccc4-6bc7-e73f-5624-e5da4a935632"),
					1421016843: NewRingMember("68943bff-6b83-476e-a613-d4ef77bf500b"),
				},
			},
			inputPolicies: []policy.PolicyID{
				"8a23e6f6-c392-4f2a-a724-c2fbb4cf4a6c",
				"dcff1218-590d-244d-1673-1e306f14339f",
				"e05c6e65-99cf-5249-c391-9624c23abecc",
				"3ec6d512-01c1-8f7b-beae-95155d03edf1",
				"edc00b7d-9e25-8257-fd53-dd8b238ba8d6",
				"ca775496-7f82-8cb6-460b-a1d19c423770",
				"d49c57a4-b700-6ceb-6a99-44384439f58e",
				"8c4738e6-6382-ae12-8181-877a7ba8403e",
				"2d197c48-094c-54c2-9885-7ba6879c8b55",
				"ef62b605-1eab-6b20-27e8-f1965c42992e",
			},
			inputAgentID: "8a52ccc4-6bc7-e73f-5624-e5da4a935632",
			expectedOutputPolicies: []policy.PolicyID{
				"8a23e6f6-c392-4f2a-a724-c2fbb4cf4a6c",
				"ef62b605-1eab-6b20-27e8-f1965c42992e",
				"e05c6e65-99cf-5249-c391-9624c23abecc",
				"2d197c48-094c-54c2-9885-7ba6879c8b55",
				"8c4738e6-6382-ae12-8181-877a7ba8403e",
				"d49c57a4-b700-6ceb-6a99-44384439f58e",
			},
			name: "general test 1",
		},
		{
			inputConsistentRing: &ConsistentRing{
				hasher:     NewDefaultHasher(),
				memberKeys: []uint32{989091847, 3928713472, 1421016843},
				ring: map[uint32]RingMember{
					989091847:  NewRingMember("04a303cf-9cd2-8faf-02e3-2ad3a387cd55"),
					3928713472: NewRingMember("8a52ccc4-6bc7-e73f-5624-e5da4a935632"),
					1421016843: NewRingMember("68943bff-6b83-476e-a613-d4ef77bf500b"),
				},
			},
			inputPolicies: []policy.PolicyID{
				"8a23e6f6-c392-4f2a-a724-c2fbb4cf4a6c",
				"dcff1218-590d-244d-1673-1e306f14339f",
				"e05c6e65-99cf-5249-c391-9624c23abecc",
				"3ec6d512-01c1-8f7b-beae-95155d03edf1",
				"edc00b7d-9e25-8257-fd53-dd8b238ba8d6",
				"ca775496-7f82-8cb6-460b-a1d19c423770",
				"d49c57a4-b700-6ceb-6a99-44384439f58e",
				"8c4738e6-6382-ae12-8181-877a7ba8403e",
				"2d197c48-094c-54c2-9885-7ba6879c8b55",
				"ef62b605-1eab-6b20-27e8-f1965c42992e",
			},
			inputAgentID: "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			expectedOutputPolicies: []policy.PolicyID{
				"dcff1218-590d-244d-1673-1e306f14339f",
				"3ec6d512-01c1-8f7b-beae-95155d03edf1",
				"edc00b7d-9e25-8257-fd53-dd8b238ba8d6",
				"ca775496-7f82-8cb6-460b-a1d19c423770",
			},
			name: "general test 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputConsistentRing.GetAssignments(tc.inputPolicies, tc.inputAgentID)
			assert.ElementsMatch(t, tc.expectedOutputPolicies, actualOutput, tc.name)

			// Test the internal load measure is correct for the local agent.
			m := &defaultRingMember{tc.inputAgentID}
			h := tc.inputConsistentRing.hasher.Hash32(m.Bytes())

			for i, memberHash := range tc.inputConsistentRing.memberKeys {
				if memberHash == h {
					assert.Equal(t, uint32(len(tc.expectedOutputPolicies)), tc.inputConsistentRing.load[i], tc.name)
					break
				}
			}
		})
	}
}

func generateRingMembers(num int) []RingMember {
	out := make([]RingMember, num)
	for i := 0; i < num; i++ {
		out[i] = NewRingMember(uuid.Generate())
	}
	return out
}
