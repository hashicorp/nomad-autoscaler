package manager

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestPluginManager_useInternal(t *testing.T) {
	l := hclog.NewNullLogger()

	testCases := []struct {
		inputPM        *PluginManager
		inputPlugin    string
		expectedOutput bool
	}{
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    internalAPMNomad,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    internalTargetNomad,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    internalAPMPrometheus,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    internalStrategyTargetValue,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    "this-plugin-doesnt-exist-either",
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, tc.inputPM.useInternal(tc.inputPlugin))
	}
}
