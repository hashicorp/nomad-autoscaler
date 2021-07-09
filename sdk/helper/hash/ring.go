package hash

import (
	"sort"
	"sync"

	"github.com/hashicorp/nomad-autoscaler/policy"
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
	memberKeys []uint32

	// ring maps the members hashed ID to the RingMember. This is used for
	// informing callers of key assignment.
	ring map[uint32]RingMember

	ringMemberLoad map[RingMember]int
	ringMembers    []RingMember
}

// NewConsistentRing returns an initialised ConsistentRing. It is safe to call
// this with a nil/empty members array, using Update to add these at a later
// time.
func NewConsistentRing() *ConsistentRing {
	return &ConsistentRing{
		hasher: NewDefaultHasher(),
	}
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

	cr.initializeWithLock(len(members))

	for i, member := range members {
		cr.addAtIndexWithLock(i, member)
	}
	cr.sortMemberKeys()
}

func (cr *ConsistentRing) initializeWithLock(num int) {
	cr.ring = make(map[uint32]RingMember, num)
	cr.memberKeys = make([]uint32, num)
	cr.ringMembers = make([]RingMember, num)
}

func (cr *ConsistentRing) addAtIndexWithLock(idx int, member RingMember) {

	h := cr.hasher.Hash32(member.Bytes())

	if _, ok := cr.ring[h]; ok {
		return
	}

	cr.ring[h] = member
	cr.memberKeys[idx] = h
	cr.ringMembers = append(cr.ringMembers, member)
}

func (cr *ConsistentRing) GetLoadStats() map[RingMember]int {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Generate the output and copy.
	out := make(map[RingMember]int, len(cr.ringMemberLoad))
	for member, load := range cr.ringMemberLoad {
		out[member] = load
	}
	return out
}

func (cr *ConsistentRing) GetAssignments(ids []policy.PolicyID, agentID string) []policy.PolicyID {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	//
	if len(cr.memberKeys) == 0 {
		return nil
	}

	var agentAssignments []policy.PolicyID

	cr.initializeLoadStatsWithLock()

	for _, policyID := range ids {

		//
		assignmentMem := cr.getMemberAssignmentWithLock(policyID.String())

		//
		if assignmentMem.String() == agentID {
			agentAssignments = append(agentAssignments, policyID)
		}

		cr.ringMemberLoad[assignmentMem]++
	}

	return agentAssignments
}

func (cr *ConsistentRing) initializeLoadStatsWithLock() {
	cr.ringMemberLoad = make(map[RingMember]int, len(cr.ringMembers))
	for _, mem := range cr.ringMembers {
		cr.ringMemberLoad[mem] = 0
	}
}

// getMemberAssignmentWithLock identifies which ring member the passed key
// should be assigned to.
func (cr *ConsistentRing) getMemberAssignmentWithLock(key string) RingMember {

	hash := cr.hasher.Hash32([]byte(key))

	// Perform a binary search to locate the appropriate member to handle the
	// key.
	idx := sort.Search(len(cr.memberKeys), func(i int) bool { return cr.memberKeys[i] >= hash })

	// If the returned index matches the length of the key array, we have
	// wrapped fully around and therefore should set the desired index to zero.
	if idx == len(cr.memberKeys) {
		idx = 0
	}

	member := cr.ring[cr.memberKeys[idx]]
	return member
}

func (cr *ConsistentRing) sortMemberKeys() {
	sort.Slice(cr.memberKeys, func(i int, j int) bool { return cr.memberKeys[i] < cr.memberKeys[j] })
}
