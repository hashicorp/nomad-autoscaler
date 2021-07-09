package hash

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

func BenchmarkConsistentRing_Update3(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 3)
}

func BenchmarkConsistentRing_Update5(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 5)
}

func BenchmarkConsistentRing_Update7(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 7)
}

func BenchmarkConsistentRing_Update9(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 9)
}

func BenchmarkConsistentRing_Update13(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 13)
}

func BenchmarkConsistentRing_Update113(b *testing.B) {
	benchmarkConsistentRingUpdate(b, 113)
}

func benchmarkConsistentRingUpdate(b *testing.B, memberCount int) {
	ring := NewConsistentRing()
	members := generateRingMembers(memberCount)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ring.Update(members)
	}
}

func BenchmarkConsistentRing_GetMemberAssignment3(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 3)
}

func BenchmarkConsistentRing_GetMemberAssignment5(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 5)
}

func BenchmarkConsistentRing_GetMemberAssignment7(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 7)
}

func BenchmarkConsistentRing_GetMemberAssignment9(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 9)
}

func BenchmarkConsistentRing_GetMemberAssignment13(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 13)
}

func BenchmarkConsistentRing_GetMemberAssignment113(b *testing.B) {
	benchmarkConsistentRingGetMemberAssignment(b, 113)
}

func benchmarkConsistentRingGetMemberAssignment(b *testing.B, memberCount int) {
	ring := NewConsistentRing()
	ring.Update(generateRingMembers(memberCount))
	keyID := uuid.Generate()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ring.getMemberAssignmentWithLock(keyID)
	}
}
