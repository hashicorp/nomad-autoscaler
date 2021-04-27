package hash

import (
	"sort"
	"sync"
)

type ConsistentRing struct {
	hasher Hasher

	keys []uint64

	mu   sync.RWMutex
	ring map[uint64]RingMember
}

func NewConsistentRing(members []RingMember) *ConsistentRing {
	cr := ConsistentRing{hasher: NewDefaultHasher()}
	cr.Update(members)
	return &cr
}

func (cr *ConsistentRing) Update(members []RingMember) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	num := len(members)

	cr.ring = make(map[uint64]RingMember, num)
	cr.keys = make([]uint64, num)

	for i, member := range members {
		cr.addAtIndexWithLock(i, member)
	}
	cr.sortKeys()
}

func (cr *ConsistentRing) addAtIndexWithLock(idx int, member RingMember) {

	h := cr.hasher.Hash64(member.Bytes())

	if _, ok := cr.ring[h]; ok {
		return
	}

	cr.ring[h] = member
	cr.keys[idx] = h
}

func (cr *ConsistentRing) GetMemberAssignment(key string) RingMember {
	if len(cr.keys) == 0 {
		return nil
	}

	hash := cr.hasher.Hash64([]byte(key))

	// Perform a binary search to locate the appropriate member to handle the
	// key.
	idx := sort.Search(len(cr.keys), func(i int) bool { return cr.keys[i] >= hash })

	// If the returned index matches the length of the key array, we have
	// wrapped fully around and therefore should set the desired index to zero.
	if idx == len(cr.keys) {
		idx = 0
	}

	member := cr.ring[cr.keys[idx]]
	return member
}

func (cr *ConsistentRing) sortKeys() {
	sort.Slice(cr.keys, func(i int, j int) bool { return cr.keys[i] < cr.keys[j] })
}
