package hash

import (
	"testing"

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

func generateRingMembers(num int) []RingMember {
	out := make([]RingMember, num)
	for i := 0; i < num; i++ {
		out[i] = NewRingMember(uuid.Generate())
	}
	return out
}
