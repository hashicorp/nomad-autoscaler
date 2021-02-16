package target

import (
	"os/exec"
	"testing"

	plugin "github.com/hashicorp/go-plugin"
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

func TestTargetPluginRPCServerSetConfig(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"target": &PluginTarget{}},
		Cmd:              exec.Command("../test/bin/noop-target"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("target")
	require.NoError(t, err)
	targetImpl := raw.(Target)

	err = targetImpl.SetConfig(map[string]string{})
	require.NoError(t, err)
}

func TestTargetPluginRPCServerPluginInfo(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"target": &PluginTarget{}},
		Cmd:              exec.Command("../test/bin/noop-target"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("target")
	require.NoError(t, err)
	targetImpl := raw.(Target)

	info, err := targetImpl.PluginInfo()
	require.NoError(t, err)
	assert.Equal(t, info.Name, "noop-target")
	assert.Equal(t, info.PluginType, "target")
}

func TestTargetPluginRPCServerStatus(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"target": &PluginTarget{}},
		Cmd:              exec.Command("../test/bin/noop-target"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("target")
	require.NoError(t, err)
	targetImpl := raw.(Target)

	status, err := targetImpl.Status(map[string]string{"count": "10", "ready": "true"})
	require.NoError(t, err)
	assert.Equal(t, int64(10), status.Count)
	assert.True(t, status.Ready)
}

func TestTargetPluginRPCServerScale(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]plugin.Plugin{"target": &PluginTarget{}},
		Cmd:              exec.Command("../test/bin/noop-target"),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	require.NoError(t, err)

	raw, err := rpcClient.Dispense("target")
	require.NoError(t, err)
	targetImpl := raw.(Target)

	err = targetImpl.Scale(sdk.ScalingAction{}, nil)
	require.NoError(t, err)
}
