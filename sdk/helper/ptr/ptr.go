// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package ptr

// Of returns a pointer to a.
func Of[A any](a A) *A {
	return &a
}
