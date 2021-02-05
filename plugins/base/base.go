package base

// Base is the common interface that all Autoscaler plugins should implement.
// It defines basic functionality which helps the Autoscaler core deal with
// plugins in a common manner.
type Base interface {

	// PluginInfo returns information regarding the plugin. This is used during
	// the plugin's setup as well as lifecycle. Any error generated during the
	// plugin's internal process to create and return this information should be
	// sent back to the caller so it can be presented.
	PluginInfo() (*PluginInfo, error)

	// SetConfig is used by the Autoscaler core to set plugin-specific
	// configuration on the remote target. If this call fails, the plugin
	// should be considered in a terminal state.
	SetConfig(config map[string]string) error
}

// PluginInfo is the information used by plugins to identify themselves and
// contains critical information about their configuration. It is used within
// the base plugin PluginInfo response RPC call.
//
// TODO (jrasell) think about moving this into the SDK.
type PluginInfo struct {
	Name       string
	PluginType string
}
