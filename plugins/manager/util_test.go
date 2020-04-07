package manager

import (
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/stretchr/testify/assert"
)

func Test_generateConfigMap(t *testing.T) {
	testCases := []struct {
		inputCfgs      map[string][]*config.Plugin
		expectedOutput map[string]map[string]*config.Plugin
	}{
		{
			inputCfgs: map[string][]*config.Plugin{
				plugins.PluginTypeAPM:      {{Name: "nomad-apm", Driver: "nomad-apm"}},
				plugins.PluginTypeStrategy: {{Name: "target-value", Driver: "target-value"}},
				plugins.PluginTypeTarget:   {{Name: "nomad-target", Driver: "nomad-target"}},
			},
			expectedOutput: map[string]map[string]*config.Plugin{
				plugins.PluginTypeAPM:      {"nomad-apm": {Name: "nomad-apm", Driver: "nomad-apm"}},
				plugins.PluginTypeStrategy: {"target-value": {Name: "target-value", Driver: "target-value"}},
				plugins.PluginTypeTarget:   {"nomad-target": {Name: "nomad-target", Driver: "nomad-target"}},
			},
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, generateConfigMap(tc.inputCfgs))
	}
}

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
