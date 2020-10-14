package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetHumanVersion(t *testing.T) {
	testCases := []struct {
		inputCommit          string
		inputDescribe        string
		inputVersion         string
		inputPrerelease      string
		inputVersionMetadata string
		expectedOutput       string
	}{
		{
			inputCommit:          "440bca3+CHANGES",
			inputDescribe:        "",
			inputVersion:         "0.1.3",
			inputPrerelease:      "dev",
			inputVersionMetadata: "",
			expectedOutput:       "v0.1.3-dev (440bca3+CHANGES)",
		},
		{
			inputCommit:          "440bca3",
			inputDescribe:        "",
			inputVersion:         "0.6.0",
			inputPrerelease:      "beta1",
			inputVersionMetadata: "",
			expectedOutput:       "v0.6.0-beta1 (440bca3)",
		},
		{
			inputCommit:          "440bca3",
			inputDescribe:        "v1.0.0",
			inputVersion:         "1.0.0",
			inputPrerelease:      "",
			inputVersionMetadata: "",
			expectedOutput:       "v1.0.0",
		},
		{
			inputCommit:          "440bca3",
			inputDescribe:        "v1.0.0",
			inputVersion:         "1.0.0",
			inputPrerelease:      "",
			inputVersionMetadata: "special",
			expectedOutput:       "v1.0.0+special",
		},
	}

	for _, tc := range testCases {
		GitCommit = tc.inputCommit
		GitDescribe = tc.inputDescribe
		Version = tc.inputVersion
		VersionPrerelease = tc.inputPrerelease
		VersionMetadata = tc.inputVersionMetadata
		assert.Equal(t, tc.expectedOutput, GetHumanVersion())
	}
}
