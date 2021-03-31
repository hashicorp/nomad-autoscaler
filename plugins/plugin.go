package plugins

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (

	// InternalAPMNomad is the Nomad APM internal plugin name.
	InternalAPMNomad = "nomad-apm"

	// InternalTargetNomad is the Nomad Target internal plugin name.
	InternalTargetNomad = "nomad-target"

	// InternalAPMPrometheus is the Prometheus APM internal plugin name.
	InternalAPMPrometheus = "prometheus"

	// InternalStrategyPassThrough is the Pass Through strategy internal plugin name.
	InternalStrategyPassThrough = "pass-through"

	// InternalStrategyTargetValue is the Target Value Strategy internal plugin
	// name.
	InternalStrategyTargetValue = "target-value"

	// InternalTargetAWSASG is the Amazon Web Services AutoScaling Group target
	// plugin.
	InternalTargetAWSASG = "aws-asg"

	// InternalTargetAzureVMSS is the Azure Virtual Machine Scale Set target
	// plugin.
	InternalTargetAzureVMSS = "azure-vmss"

	// InternalTargetGCEMIG is the Google Compute Engine Managed Instance Group target
	// plugin.
	InternalTargetGCEMIG = "gce-mig"

	// InternalAPMDatadog is the Datadog APM plugin name.
	InternalAPMDatadog = "datadog"
)

// ConfigKeyNomadConfigInherit is a generic plugin config map key that supports
// a boolean value. It indicates whether or not the plugin config should be
// merged with the agent's Nomad config. This provides an easy simple way in
// which plugins can have their Nomad client configured without extra hassle.
const ConfigKeyNomadConfigInherit = "nomad_config_inherit"

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

// Serve is used to serve a Nomad Autoscaler Base.
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
		GRPCServer:      plugin.DefaultGRPCServer,
	}

	switch pType := p.(type) {
	case apm.APM:
		pCfg.Plugins = map[string]plugin.Plugin{
			sdk.PluginTypeAPM:  &apm.PluginAPM{Impl: p.(apm.APM)},
			sdk.PluginTypeBase: &base.PluginBase{Impl: p.(apm.APM)},
		}
	case target.Target:
		pCfg.Plugins = map[string]plugin.Plugin{
			sdk.PluginTypeTarget: &target.PluginTarget{Impl: p.(target.Target)},
			sdk.PluginTypeBase:   &base.PluginBase{Impl: p.(target.Target)},
		}
	case strategy.Strategy:
		pCfg.Plugins = map[string]plugin.Plugin{
			sdk.PluginTypeStrategy: &strategy.PluginStrategy{Impl: p.(strategy.Strategy)},
			sdk.PluginTypeBase:     &base.PluginBase{Impl: p.(strategy.Strategy)},
		}
	default:
		logger.Error("unsupported plugin type %q", pType)
		return
	}

	// Serve the plugin; lovely jubbly.
	plugin.Serve(&pCfg)
}
