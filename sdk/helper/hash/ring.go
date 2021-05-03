package hash

import (
	"fmt"
	"sort"
	"sync"
)

// ConsistentRing is a basic implementation of a consistent hash ring providing
// consistent assignment of keys to ring members.
type ConsistentRing struct {
	hasher Hasher

	// mu should be used to ensure ring consistency when updating or
	// identifying assignments. In the search for stronger consistency this is
	// used to interact with all objects in this struct below this definition.
	mu sync.RWMutex

	// memberKeys is the sorted array of hashed member IDs. Whenever the array
	// is updated, it should be sorted using sortMemberKeys.
	memberKeys []uint64

	// ring maps the members hashed ID to the RingMember. This is used for
	// informing callers of key assignment.
	ring map[uint64]RingMember

	ringMemberLoad map[RingMember]int
	keyAssignment  map[uint64]RingMember
}

// NewConsistentRing returns an initialised ConsistentRing. It is safe to call
// this with a nil/empty members array, using Update to add these at a later
// time.
func NewConsistentRing(members []RingMember) *ConsistentRing {
	cr := ConsistentRing{
		hasher: NewDefaultHasher(),
	}
	cr.Update(members)
	return &cr
}

// Update is used to overwrite the current ring membership with an updated list
// of members. This method is used currently in-place of singular add/remove
// functions due to the way in which Consul allows us to get notified of
// service catalog changes.
func (cr *ConsistentRing) Update(members []RingMember) {

	// It is possible NewConsistentRing will be called with no initial members
	// therefore exit early if possible.
	if len(members) < 1 {
		return
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	num := len(members)

	cr.keyAssignment = make(map[uint64]RingMember)
	cr.ringMemberLoad = make(map[RingMember]int, num)

	cr.ring = make(map[uint64]RingMember, num)
	cr.memberKeys = make([]uint64, num)

	for i, member := range members {
		cr.addAtIndexWithLock(i, member)
	}
	cr.sortMemberKeys()
}

func (cr *ConsistentRing) addAtIndexWithLock(idx int, member RingMember) {

	h := cr.hasher.Hash64(member.Bytes())

	if _, ok := cr.ring[h]; ok {
		return
	}

	cr.ring[h] = member
	cr.memberKeys[idx] = h
}

// GetMemberAssignment identifies which ring member the passed key should be
// assigned to. In the event that there are no ring members, a nil RingMember
// will be returned. The caller should ensure this is checked.
func (cr *ConsistentRing) GetMemberAssignment(key string) RingMember {
	if len(cr.memberKeys) == 0 {
		return nil
	}

	hash := cr.hasher.Hash64([]byte(key))

	// Perform a binary search to locate the appropriate member to handle the
	// key.
	idx := sort.Search(len(cr.memberKeys), func(i int) bool { return cr.memberKeys[i] >= hash })

	// If the returned index matches the length of the key array, we have
	// wrapped fully around and therefore should set the desired index to zero.
	if idx == len(cr.memberKeys) {
		idx = 0
	}

	cr.mu.RLock()
	defer cr.mu.RUnlock()

	member := cr.ring[cr.memberKeys[idx]]
	cr.updateLoadWithLock(hash, member)

	return member
}

func (cr *ConsistentRing) EmitLoadStats() {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	for mem, load := range cr.ringMemberLoad {
		fmt.Println(fmt.Sprintf("member ID %q loaded with %v keys", mem, load))
	}
}

func (cr *ConsistentRing) updateLoadWithLock(keyHash uint64, member RingMember) {

	prevMem, ok := cr.keyAssignment[keyHash]
	if !ok {
		cr.keyAssignment[keyHash] = member
		cr.ringMemberLoad[member]++
		return
	}

	if prevMem == member {
		return
	}

	cr.keyAssignment[keyHash] = member
	cr.ringMemberLoad[member]++
	cr.ringMemberLoad[prevMem]--
}

func (cr *ConsistentRing) sortMemberKeys() {
	sort.Slice(cr.memberKeys, func(i int, j int) bool { return cr.memberKeys[i] < cr.memberKeys[j] })
}
