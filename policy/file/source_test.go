package file

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/stretchr/testify/assert"
)

func TestSource_getFilePolicyID(t *testing.T) {
	testCases := []struct {
		inputFile   string
		existingID  policy.PolicyID
		inputSource *Source
		name        string
	}{
		{
			inputFile:  "/this/test/file.hcl",
			existingID: "587a5903-f77b-c7e6-a07a-def5af92f791",
			inputSource: &Source{idMap: map[pathMD5Sum]policy.PolicyID{
				md5Sum("/this/test/file.hcl"): "587a5903-f77b-c7e6-a07a-def5af92f791",
			}},
			name: "file already within idMap",
		},

		{
			inputFile:   "/this/test/file.hcl",
			existingID:  "",
			inputSource: &Source{idMap: map[pathMD5Sum]policy.PolicyID{}},
			name:        "file not within idMap",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputID := tc.inputSource.getFilePolicyID(tc.inputFile)

			if tc.existingID != "" {
				assert.Equal(t, tc.existingID, outputID, tc.name)
			}

			policyID, ok := tc.inputSource.idMap[md5Sum(tc.inputFile)]
			assert.Equal(t, outputID, policyID, tc.name)
			assert.True(t, ok, tc.name)
		})
	}
}
