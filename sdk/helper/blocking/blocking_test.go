// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package blocking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_indexHasChange(t *testing.T) {
	testCases := []struct {
		newValue       uint64
		oldValue       uint64
		expectedReturn bool
	}{
		{
			newValue:       13,
			oldValue:       7,
			expectedReturn: true,
		},
		{
			newValue:       13696,
			oldValue:       13696,
			expectedReturn: false,
		},
		{
			newValue:       7,
			oldValue:       13,
			expectedReturn: true,
		},
	}

	for _, tc := range testCases {
		res := IndexHasChanged(tc.newValue, tc.oldValue)
		assert.Equal(t, tc.expectedReturn, res)
	}
}
