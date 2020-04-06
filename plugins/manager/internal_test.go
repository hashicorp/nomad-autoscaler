package manager

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
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
			inputPlugin:    plugins.InternalAPMNomad,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    plugins.InternalTargetNomad,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    plugins.InternalAPMPrometheus,
			expectedOutput: true,
		},
		{
			inputPM:        NewPluginManager(l, "this/doesnt/exist", nil),
			inputPlugin:    plugins.InternalStrategyTargetValue,
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
