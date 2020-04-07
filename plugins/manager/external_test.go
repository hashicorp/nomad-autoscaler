package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cleanPluginExecutable(t *testing.T) {
	testCases := []struct {
		inputName      string
		expectedOutput string
	}{
		{inputName: "normal-looking-file", expectedOutput: "normal-looking-file"},
		{inputName: "windows-exe-file.exe", expectedOutput: "windows-exe-file"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, cleanPluginExecutable(tc.inputName))
	}
}
