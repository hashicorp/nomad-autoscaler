// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package modes

// set stores unique values.
// This struct is not safe for concurrent access.
type set struct {
	storage map[string]bool
}

// newSet returns a new set object.
// Initial values can be passed as the function arguments.
func newSet(values ...string) *set {
	set := &set{
		storage: make(map[string]bool),
	}

	set.add(values...)
	return set
}

// add adds a new element to the set.
// Panics if the set is nil.
// Empty inputs are ignored.
func (s *set) add(in ...string) {
	for _, i := range in {
		if len(i) > 0 {
			s.storage[i] = true
		}
	}
}

// contains returns true if the set has element.
// Returns false if set is nil.
func (s *set) contains(v string) bool {
	if s == nil {
		return false
	}

	_, ok := s.storage[v]
	return ok
}

// len returns the number of elements in the set.
// Returns 0 if set is nil.
func (s *set) len() int {
	if s == nil {
		return 0
	}

	return len(s.storage)
}

// merge adds the values from the input set into its own values.
func (s *set) merge(in *set) {
	if in == nil || s == nil {
		return
	}

	s.add(in.values()...)
}

// values returns a slice with the values stored in the set.
// Returns an empty slice is set is nil.
// Order is not guaranteed.
func (s *set) values() []string {
	values := []string{}

	if s == nil {
		return values
	}

	for k := range s.storage {
		values = append(values, k)
	}
	return values
}
