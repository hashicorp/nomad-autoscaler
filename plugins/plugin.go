package plugins

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

const (
	// PluginTypeAPM is a plugin which satisfies the APM interface.
	PluginTypeAPM = "apm"

	// PluginTypeTarget is a plugin which satisfies the Target interface.
	PluginTypeTarget = "target"

	// PluginTypeStrategy is a plugin which satisfies the Strategy interface.
	PluginTypeStrategy = "strategy"
)

var (
	// Handshake is used to do a basic handshake between a plugin and host. If
	// the handshake fails, a user friendly error is shown. This prevents users
	// from executing bad plugins or executing a plugin directory. It is a UX
	// feature, not a security feature.
	//
	// Currently the Nomad Autoscaler plugin visioning is performed using the
	// ProtocolVersion within the Handshake.
	Handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "NOMAD_AUTOSCALER_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "e082fa04d587a6525d683666fa253d6afda00f20c122c54a80a3ed57fec99ff3",
	}
)

// PluginFactory is used to return a new plugin instance.
type PluginFactory func(log hclog.Logger) interface{}

// InternalPluginConfig is a struct that internal plugins must implement in
// order to provide critical information about the plugin and launching it.
type InternalPluginConfig struct{ Factory PluginFactory }

// PluginID contains plugin metadata and identifies a plugin. It is used as a
// key to aid plugin lookups. The information held here is also reflected
// within PluginInfo, but will not include all.
type PluginID struct {
	Name       string
	PluginType string
}

// String returns a human readable version of PluginID.
func (p PluginID) String() string {
	return fmt.Sprintf("%q (%v)", p.Name, p.PluginType)
}

// PluginInfo is the information used by plugins to identify themselves and
// contains critical information about their configuration. It is used within
// the base plugin PluginInfo response RPC call.
type PluginInfo struct {
	Name       string
	PluginType string
}
