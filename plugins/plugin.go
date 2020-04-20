package plugins

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
)

const (
	// PluginTypeAPM is a plugin which satisfies the APM interface.
	PluginTypeAPM = "apm"

	// PluginTypeTarget is a plugin which satisfies the Target interface.
	PluginTypeTarget = "target"

	// PluginTypeStrategy is a plugin which satisfies the Strategy interface.
	PluginTypeStrategy = "strategy"
)

const (

	// InternalAPMNomad is the Nomad APM internal plugin name.
	InternalAPMNomad = "nomad-apm"

	// InternalTargetNomad is the Nomad Target internal plugin name.
	InternalTargetNomad = "nomad-target"

	// InternalAPMPrometheus is the Prometheus APM internal plugin name.
	InternalAPMPrometheus = "prometheus"

	// InternalStrategyTargetValue is the Target Value Strategy internal plugin
	// name.
	InternalStrategyTargetValue = "target-value"
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

// Serve is used to serve a Nomad Autoscaler Plugin.
func Serve(f PluginFactory) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		JSONFormat: true,
	})

	// Generate the plugin.
	p := f(logger)
	if p == nil {
		logger.Error("plugin factory returned nil")
		return
	}

	// Build the base plugin configuration which is independent of the plugin
	// type.
	pCfg := plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Logger:          logger,
	}

	switch pType := p.(type) {
	case apm.APM:
		pCfg.Plugins = map[string]plugin.Plugin{PluginTypeAPM: &apm.Plugin{Impl: p.(apm.APM)}}
	case target.Target:
		pCfg.Plugins = map[string]plugin.Plugin{PluginTypeTarget: &target.Plugin{Impl: p.(target.Target)}}
	case strategy.Strategy:
		pCfg.Plugins = map[string]plugin.Plugin{PluginTypeStrategy: &strategy.Plugin{Impl: p.(strategy.Strategy)}}
	default:
		logger.Error("unsupported plugin type %q", pType)
		return
	}

	// Serve the plugin; lovely jubbly.
	plugin.Serve(&pCfg)
}
