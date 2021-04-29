package hash

import "github.com/cespare/xxhash/v2"

// Hasher is responsible for generating unsigned, 64 bit hash of provided byte
// array.
type Hasher interface {

	// Hash64 takes an input byte array and returns a hashed version.
	Hash64([]byte) uint64
}

// xxHasher is the xxhash 64-bit implementation of the Hasher interface and
// generates hashes according to http://cyan4973.github.io/xxHash/.
type xxHasher struct{}

// NewDefaultHasher returns a new xxhasher implementation of the Hasher
// interface.
func NewDefaultHasher() Hasher { return xxHasher{} }

// Hash64 satisfies the Hash64 function on the Hasher interface.
func (x xxHasher) Hash64(key []byte) uint64 { return xxhash.Sum64(key) }
