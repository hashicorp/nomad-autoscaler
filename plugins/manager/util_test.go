package manager

import (
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func Test_getPluginMap(t *testing.T) {
	testCases := []struct {
		inputPluginType string
		expectedOutput  map[string]plugin.Plugin
	}{
		{
			inputPluginType: sdk.PluginTypeAPM,
			expectedOutput: map[string]plugin.Plugin{
				sdk.PluginTypeAPM:  &apm.PluginAPM{},
				sdk.PluginTypeBase: &base.PluginBase{},
			},
		},
		{
			inputPluginType: sdk.PluginTypeTarget,
			expectedOutput: map[string]plugin.Plugin{
				sdk.PluginTypeTarget: &target.PluginTarget{},
				sdk.PluginTypeBase:   &base.PluginBase{},
			},
		},
		{
			inputPluginType: sdk.PluginTypeStrategy,
			expectedOutput: map[string]plugin.Plugin{
				sdk.PluginTypeStrategy: &strategy.PluginStrategy{},
				sdk.PluginTypeBase:     &base.PluginBase{},
			},
		},
		{
			inputPluginType: "automatic-pizza-delivery",
			expectedOutput:  map[string]plugin.Plugin{sdk.PluginTypeBase: &base.PluginBase{}},
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedOutput, getPluginMap(tc.inputPluginType))
	}
}
