package hash

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_xxhasher(t *testing.T) {
	testCases := []struct {
		name string
		f    func() error
	}{
		{
			name: "ensure same key same hash",
			f: func() error {
				hashFunc := NewDefaultHasher()
				key := []byte("nomad-autoscaler")
				hash1 := hashFunc.Hash64(key)
				hash2 := hashFunc.Hash64(key)
				if hash1 != hash2 {
					return fmt.Errorf("expected same hashes, got %v and %v", hash1, hash2)
				}
				return nil
			},
		},
		{
			name: "ensure different key different hash",
			f: func() error {
				hashFunc := NewDefaultHasher()
				key1 := []byte("nomad-autoscaler-1")
				key2 := []byte("nomad-autoscaler-2")
				hash1 := hashFunc.Hash64(key1)
				hash2 := hashFunc.Hash64(key2)
				if hash1 == hash2 {
					return fmt.Errorf("expected different hashes, got %v and %v", hash1, hash2)
				}
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Nil(t, tc.f(), tc.name)
		})
	}
}
