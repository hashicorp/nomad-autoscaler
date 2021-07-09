package hash

// RingMember is an interface that defines how a consistent hash ring member
// must be implemented. The functionality is minimal but provides an ease of
// functionality and consistency within the ring code.
type RingMember interface {

	// Bytes returns the ring member ID as a byte array which is useful when
	// converting the member ID into a hash.
	Bytes() []byte

	// String returns a string represent of the member ID, useful for human
	// consumption.
	String() string
}

// defaultRingMember is the default implementation of RingMember and uses an ID
// passed on initialization to create the outputs.
type defaultRingMember struct {
	id string
}

// NewRingMember returns a new defaultRingMember implementation of the
// RingMember interface.
func NewRingMember(id string) RingMember {
	if id == "" {
		panic("ring member ID cannot to empty")
	}
	return &defaultRingMember{
		id: id,
	}
}

// Bytes satisfies the Bytes function on the RingMember interface.
func (d *defaultRingMember) Bytes() []byte { return []byte(d.id) }

// String satisfies the String function on the RingMember interface.
func (d *defaultRingMember) String() string { return d.id }
