package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultRingMember(t *testing.T) {
	testCases := []struct {
		inputID             string
		expectedOutputBytes []byte
		name                string
	}{
		{
			inputID:             "nomad-autoscaler-3a102984-a732-11eb-bcbc-0242ac130002",
			expectedOutputBytes: []byte("nomad-autoscaler-3a102984-a732-11eb-bcbc-0242ac130002"),
			name:                "generic test 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ringMem := NewRingMember(tc.inputID)
			assert.Equal(t, tc.inputID, ringMem.String(), tc.name)
			assert.Equal(t, tc.expectedOutputBytes, ringMem.Bytes(), tc.name)
		})
	}
}
