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
	// is updated, it should be sorted using sortMemberKeys and assignments
	// re-calculated.
	memberKeys []uint32

	// ring maps the members hashed ID to the RingMember. This is used for
	// informing callers of key assignment.
	ring map[uint32]RingMember

	// load tracks the number of policies assigned per member. The index is
	// mirrored from the memberKeys, therefore the load[2] represent the load
	// of member memberKeys[2] and ring[memberKeys[2]].
	load []uint32
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

	num := len(members)

	if num < 1 {
		return
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// This is a bulk update method, therefore init all the things.
	cr.ring = make(map[uint32]RingMember, num)
	cr.memberKeys = make([]uint32, num)
	cr.load = make([]uint32, num)

	// Iterate the members and insert them into the ring objects so we can
	// correctly sort the ring and distribute work.
	for i, mem := range members {
		h := cr.hasher.Hash32(mem.Bytes())
		cr.ring[h] = mem
		cr.memberKeys[i] = h
	}

	// Don't forget to sort!
	cr.sortMemberKeys()
}

func (cr *ConsistentRing) GetLoadStats() {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	// Implement me, please.
}

func (cr *ConsistentRing) GetAssignments(ids []policy.PolicyID, agentID string) []policy.PolicyID {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Guard against attempting to assign policies where we do not have any
	// members.
	if len(cr.memberKeys) == 0 {
		return nil
	}

	// This function bulk assigns work therefore we can overwrite the load
	// tracking.
	cr.load = make([]uint32, len(cr.memberKeys))

	// The base length allows us to mutate the iteration loop depending whether
	// we remove elements.
	numIDs := len(ids)

	for i := 0; i < numIDs; i++ {

		// Grab the policy and identify which member this will be assigned to.
		p := ids[i]
		assignmentMem := cr.getMemberAssignmentWithLock(p.String())

		if assignmentMem.String() != agentID {

			// Remove the policy from the list if it isn't assigned to the
			// local agent.
			ids[numIDs-1], ids[i] = ids[i], ids[numIDs-1]
			ids = ids[:numIDs-1]

			// The array size has shrunk by one, therefore keep the iteration
			// point static and reduce the effective end of the iteration.
			i--
			numIDs--
		}
	}

	return ids
}

// getMemberAssignmentWithLock identifies which ring member the passed key
// should be assigned to.
func (cr *ConsistentRing) getMemberAssignmentWithLock(key string) RingMember {

	// The fact we have to convert the policyID from a string to a byte array
	// accounts for a large amount time comparable to the work being done. This
	// is seen within the cpu profile for the benchmarks particularly when
	// running GetAssignments().
	hash := cr.hasher.Hash32([]byte(key))

	// Perform a binary search to locate the appropriate member to handle the
	// key.
	idx := sort.Search(len(cr.memberKeys), func(i int) bool { return cr.memberKeys[i] >= hash })

	// If the returned index matches the length of the key array, we have
	// wrapped fully around and therefore should set the desired index to zero.
	if idx == len(cr.memberKeys) {
		idx = 0
	}

	// Update our load tracking. This is the most efficient place to do this,
	// although it is not the most flexible. If we ever update the discovery
	// method this will likely need re-thinking. That being said, for now,
	// simple and easy.
	cr.load[idx]++

	// Grab the member from the Ring using the index and return.
	return cr.ring[cr.memberKeys[idx]]
}

// sortMemberKeys ensures the memberKeys, and therefore our ring, is ordered so
// we can provide consistent responses to assignment requests. This needs to be
// called on every update to ConsistentRing membership.
func (cr *ConsistentRing) sortMemberKeys() {
	sort.Slice(cr.memberKeys, func(i int, j int) bool { return cr.memberKeys[i] < cr.memberKeys[j] })
}
