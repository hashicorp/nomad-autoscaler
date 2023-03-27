// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsTemporaryFile(t *testing.T) {
	testCases := []struct {
		testName       string
		inputName      string
		expectedReturn bool
	}{
		{
			testName:       "vim temp input file",
			inputName:      "config.hcl~",
			expectedReturn: true,
		},
		{
			testName:       "emacs temp input file 1",
			inputName:      ".#config.hcl",
			expectedReturn: true,
		},
		{
			testName:       "emacs temp input file 2",
			inputName:      "#config.hcl#",
			expectedReturn: true,
		},
		{
			testName:       "non-temp HCL config input file",
			inputName:      "config.hcl",
			expectedReturn: false,
		},
		{
			testName:       "non-temp JSON config input file",
			inputName:      "config.json",
			expectedReturn: false,
		},
	}

	for _, tc := range testCases {
		actualOutput := IsTemporaryFile(tc.inputName)
		assert.Equal(t, tc.expectedReturn, actualOutput, tc.testName)
	}
}
