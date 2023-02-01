// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package uuid

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	testCases := []struct {
		count int
		name  string
	}{
		{count: 100, name: "generate 100 unique uuids"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			stored := make(map[string]interface{})

			compiledRegex, err := regexp.Compile(`[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}`)
			assert.Nil(t, err, tc.name)
			assert.NotNil(t, compiledRegex, tc.name)

			for i := 0; i < 100; i++ {

				newUUID := Generate()

				// Check the UUID matches the expected regex.
				matched := compiledRegex.MatchString(newUUID)
				assert.True(t, matched, tc.name)

				// Ensure we have not seen the UUID before. Then store the new
				// UUID.
				val, ok := stored[newUUID]
				assert.False(t, ok, tc.name)
				assert.Nil(t, val, tc.name)
				stored[newUUID] = nil
			}
		})
	}
}
