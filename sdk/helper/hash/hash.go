package hash

import (
	"hash/crc32"
)

// Hasher is responsible for generating unsigned, 32 bit hash of provided byte
// array.
type Hasher interface {

	// Hash32 takes an input byte array and returns a hashed version.
	Hash32([]byte) uint32
}

// defaultHasher is the default hasher that utilises crc32.ChecksumIEEE.
type defaultHasher struct{}

// NewDefaultHasher returns a new defaultHasher implementation of the Hasher
// interface.
func NewDefaultHasher() Hasher { return defaultHasher{} }

// Hash32 satisfies the Hash32 function on the Hasher interface.
func (d defaultHasher) Hash32(key []byte) uint32 { return crc32.ChecksumIEEE(key) }
