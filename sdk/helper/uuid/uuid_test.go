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

			for i := 0; i < 100; i++ {

				newUUID := Generate()

				// Check the UUID matches the expected regex.
				matched, err := regexp.MatchString("[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}", newUUID)
				assert.True(t, matched, tc.name)
				assert.Nil(t, err, tc.name)

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
