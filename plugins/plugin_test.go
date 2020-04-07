package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginID_String(t *testing.T) {
	testCases := []struct {
		inputPluginID  PluginID
		expectedOutput string
	}{
		{
			inputPluginID: PluginID{
				Name:       "foobar",
				PluginType: PluginTypeAPM,
			},
			expectedOutput: "\"foobar\" (apm)",
		},
		{
			inputPluginID: PluginID{
				Name:       "foobar",
				PluginType: PluginTypeTarget,
			},
			expectedOutput: "\"foobar\" (target)",
		},
		{
			inputPluginID: PluginID{
				Name:       "foobar",
				PluginType: PluginTypeStrategy,
			},
			expectedOutput: "\"foobar\" (strategy)",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, tc.inputPluginID.String())
	}
}
