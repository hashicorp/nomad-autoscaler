// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package strategy

import (
	"os/exec"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO(luiz): there's an import cycle, so let's copy it here for now.
var handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NOMAD_AUTOSCALER_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "e082fa04d587a6525d683666fa253d6afda00f20c122c54a80a3ed57fec99ff3",
}

func TestStrategyPluginRPCServerSetConfig(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"strategy": &PluginStrategy{}},
		Cmd:              exec.Command("../test/bin/noop-strategy"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("strategy")
	require.NoError(t, err)
	strategyImpl := raw.(Strategy)

	err = strategyImpl.SetConfig(map[string]string{})
	require.NoError(t, err)
}

func TestStrategyPluginRPCServerPluginInfo(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"strategy": &PluginStrategy{}},
		Cmd:              exec.Command("../test/bin/noop-strategy"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("strategy")
	require.NoError(t, err)
	strategyImpl := raw.(Strategy)

	info, err := strategyImpl.PluginInfo()
	require.NoError(t, err)
	assert.Equal(t, info.Name, "noop-strategy")
	assert.Equal(t, info.PluginType, "strategy")
}

func TestStrategyPluginRPCServerRun(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"strategy": &PluginStrategy{}},
		Cmd:              exec.Command("../test/bin/noop-strategy"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("strategy")
	require.NoError(t, err)
	strategyImpl := raw.(Strategy)

	inputEval := &sdk.ScalingCheckEvaluation{
		Check: &sdk.ScalingPolicyCheck{
			Strategy: &sdk.ScalingPolicyStrategy{
				Config: map[string]string{
					"count": "5",
				},
			},
		},
	}

	resultEval, err := strategyImpl.Run(inputEval, 0)
	require.NoError(t, err)
	assert.NotNil(t, resultEval)
	assert.Equal(t, int64(5), resultEval.Action.Count)
}
