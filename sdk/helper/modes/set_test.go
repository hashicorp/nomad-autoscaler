// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package modes

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet_newSet(t *testing.T) {
	testCases := []struct {
		name     string
		in       []string
		expected *set
	}{
		{
			name:     "nil",
			in:       nil,
			expected: &set{storage: make(map[string]bool)},
		},
		{
			name:     "empty",
			in:       []string{},
			expected: &set{storage: make(map[string]bool)},
		},
		{
			name:     "empty value",
			in:       []string{""},
			expected: &set{storage: make(map[string]bool)},
		},
		{
			name:     "initialized",
			in:       []string{"a", "b", "c"},
			expected: &set{storage: map[string]bool{"a": true, "b": true, "c": true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := newSet(tc.in...)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSet_add(t *testing.T) {
	// Panics if set is nil.
	t.Run("panic on nil", func(t *testing.T) {
		assert.Panics(t, func() {
			var s *set
			s.add("a")
		})
	})

	testCases := []struct {
		name     string
		s        *set
		in       []string
		expected *set
	}{
		{
			name:     "add to empty",
			s:        newSet(),
			in:       []string{"a", "b"},
			expected: newSet("a", "b"),
		},
		{
			name:     "add with existing",
			s:        newSet("a", "b"),
			in:       []string{"c"},
			expected: newSet("a", "b", "c"),
		},
		{
			name:     "no repeat",
			s:        newSet("a"),
			in:       []string{"a", "a", "a"},
			expected: newSet("a"),
		},
		{
			name:     "add empty",
			s:        newSet("a"),
			in:       []string{},
			expected: newSet("a"),
		},
		{
			name:     "add empty value",
			s:        newSet("a"),
			in:       []string{""},
			expected: newSet("a"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.s.add(tc.in...)
			assert.Equal(t, tc.expected, tc.s)
		})
	}
}

func TestSet_contains(t *testing.T) {
	testCases := []struct {
		name     string
		s        *set
		value    string
		expected bool
	}{
		{
			name:     "nil",
			s:        nil,
			value:    "a",
			expected: false,
		},
		{
			name:     "empty set",
			s:        newSet(),
			value:    "a",
			expected: false,
		},
		{
			name:     "empty set and input",
			s:        newSet(),
			value:    "",
			expected: false,
		},
		{
			name:     "empty input",
			s:        newSet("a"),
			value:    "",
			expected: false,
		},
		{
			name:     "has value",
			s:        newSet("a", "b", "c"),
			value:    "a",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.s.contains(tc.value)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSet_len(t *testing.T) {
	testCases := []struct {
		name     string
		s        *set
		expected int
	}{
		{
			name:     "nil",
			s:        nil,
			expected: 0,
		},
		{
			name:     "empty",
			s:        newSet(),
			expected: 0,
		},
		{
			name:     "one",
			s:        newSet("a"),
			expected: 1,
		},
		{
			name:     "many",
			s:        newSet("a", "b", "c"),
			expected: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.s.len()
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSet_merge(t *testing.T) {
	// Make sure we don't mutate input set.
	t.Run("no mutate", func(t *testing.T) {
		initialValues := map[string]bool{"a": true, "b": true}
		s1 := newSet("a", "b")
		s2 := newSet("c")

		s2.merge(s1)
		assert.Equal(t, newSet("a", "b", "c"), s2)
		assert.Equal(t, initialValues, s1.storage)
	})

	testCases := []struct {
		name     string
		s1       *set
		s2       *set
		expected *set
	}{
		{
			name:     "nil target",
			s1:       newSet("a"),
			s2:       nil,
			expected: nil,
		},
		{
			name:     "nil input",
			s1:       nil,
			s2:       newSet("a"),
			expected: newSet("a"),
		},
		{
			name:     "both nil",
			s1:       nil,
			s2:       nil,
			expected: nil,
		},
		{
			name:     "empty target",
			s1:       newSet("a"),
			s2:       newSet(),
			expected: newSet("a"),
		},
		{
			name:     "empty input",
			s1:       newSet(),
			s2:       newSet("a"),
			expected: newSet("a"),
		},
		{
			name:     "merge multiple",
			s1:       newSet("a", "b"),
			s2:       newSet("c"),
			expected: newSet("a", "b", "c"),
		},
		{
			name:     "merge into multiple",
			s1:       newSet("c"),
			s2:       newSet("a", "b"),
			expected: newSet("a", "b", "c"),
		},
		{
			name:     "merge repeated",
			s1:       newSet("a", "b"),
			s2:       newSet("b", "c"),
			expected: newSet("a", "b", "c"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.s2.merge(tc.s1)
			assert.Equal(t, tc.expected, tc.s2)
		})
	}
}

func TestSet_values(t *testing.T) {
	testCases := []struct {
		name     string
		s        *set
		expected []string
	}{
		{
			name:     "nil",
			s:        nil,
			expected: []string{},
		},
		{
			name:     "empty",
			s:        newSet(),
			expected: []string{},
		},
		{
			name:     "one element",
			s:        newSet("a"),
			expected: []string{"a"},
		},
		{
			name:     "multiple elements",
			s:        newSet("a", "b", "c"),
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "repeated elements",
			s:        newSet("a", "a", "b", "c"),
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty element",
			s:        newSet("a", "", "c"),
			expected: []string{"a", "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.s.values()

			// Sort values to guarantee order.
			sort.Strings(tc.expected)
			sort.Strings(got)

			assert.Equal(t, tc.expected, got)
		})
	}
}
