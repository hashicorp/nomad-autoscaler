package manager

import (
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/stretchr/testify/assert"
)

func Test_getPluginMap(t *testing.T) {
	testCases := []struct {
		inputPluginType string
		expectedOutput  map[string]plugin.Plugin
	}{
		{
			inputPluginType: plugins.PluginTypeAPM,
			expectedOutput:  map[string]plugin.Plugin{plugins.PluginTypeAPM: &apm.Plugin{}},
		},
		{
			inputPluginType: plugins.PluginTypeTarget,
			expectedOutput:  map[string]plugin.Plugin{plugins.PluginTypeTarget: &target.Plugin{}},
		},
		{
			inputPluginType: plugins.PluginTypeStrategy,
			expectedOutput:  map[string]plugin.Plugin{plugins.PluginTypeStrategy: &strategy.Plugin{}},
		},
		{
			inputPluginType: "automatic-pizza-delivery",
			expectedOutput:  map[string]plugin.Plugin{},
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, getPluginMap(tc.inputPluginType))
	}
}
