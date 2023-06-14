// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/source"
	"github.com/stretchr/testify/assert"
)

func TestSource_getFilePolicyID(t *testing.T) {
	testCases := []struct {
		inputFile       string
		inputPolicyName string
		existingID      source.PolicyID
		inputSource     *FileSource
		name            string
	}{
		{
			inputFile:       "/this/test/file.hcl",
			inputPolicyName: "policy_name",
			existingID:      "b65aa225-35bd-aa72-29d0-a1d228662817",
			inputSource: &FileSource{idMap: map[pathMD5Sum]source.PolicyID{
				md5Sum("/this/test/file.hcl/policy_name"): "b65aa225-35bd-aa72-29d0-a1d228662817",
			}},
			name: "file already within idMap",
		},

		{
			inputFile:       "/this/test/file.hcl",
			inputPolicyName: "policy_name",
			existingID:      "",
			inputSource:     &FileSource{idMap: map[pathMD5Sum]source.PolicyID{}},
			name:            "file not within idMap",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputID := tc.inputSource.getFilePolicyID(tc.inputFile, tc.inputPolicyName)

			if tc.existingID != "" {
				assert.Equal(t, tc.existingID, outputID, tc.name)
			}

			policyID, ok := tc.inputSource.idMap[md5Sum(tc.inputFile+"/"+tc.inputPolicyName)]
			assert.Equal(t, policyID, outputID, tc.name)
			assert.True(t, ok, tc.name)
		})
	}
}
