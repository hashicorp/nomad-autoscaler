// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package apm

import (
	"os/exec"
	"testing"
	"time"

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

func TestAPMPluginRPCServerSetConfig(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"apm": &PluginAPM{}},
		Cmd:              exec.Command("../test/bin/noop-apm"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("apm")
	require.NoError(t, err)
	apmImpl := raw.(APM)

	err = apmImpl.SetConfig(map[string]string{})
	require.NoError(t, err)
}

func TestAPMPluginRPCServerPluginInfo(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"apm": &PluginAPM{}},
		Cmd:              exec.Command("../test/bin/noop-apm"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("apm")
	require.NoError(t, err)
	apmImpl := raw.(APM)

	info, err := apmImpl.PluginInfo()
	require.NoError(t, err)
	assert.Equal(t, info.Name, "noop-apm")
	assert.Equal(t, info.PluginType, "apm")
}

func TestAPMPluginRPCServerQuery(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"apm": &PluginAPM{}},
		Cmd:              exec.Command("../test/bin/noop-apm"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("apm")
	require.NoError(t, err)
	apmImpl := raw.(APM)

	now := time.Now()
	r := sdk.TimeRange{From: now.Add(-10 * time.Second), To: now}

	result, err := apmImpl.Query("fixed:5", r)
	require.NoError(t, err)
	assert.Len(t, result, 10)
}

func TestAPMPluginRPCServerQueryMultiple(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"apm": &PluginAPM{}},
		Cmd:              exec.Command("../test/bin/noop-apm"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("apm")
	require.NoError(t, err)
	apmImpl := raw.(APM)

	now := time.Now()
	r := sdk.TimeRange{From: now.Add(-10 * time.Second), To: now}

	result, err := apmImpl.QueryMultiple("fixed:5", r)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0], 10)
}
