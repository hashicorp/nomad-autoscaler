// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package manager

import plugin "github.com/hashicorp/go-plugin"

// PluginInstance is a wrapper of a plugin and provides a common interface
// whether the plugin is internal or running externally via a binary.
type PluginInstance interface {

	// Kill kills the plugin if it is external. It is safe to call on internal
	// plugins.
	Kill()

	// Plugin returns the wrapped plugin instance.
	Plugin() interface{}
}

// internalPluginInstance wraps an internal plugin.
type internalPluginInstance struct {
	instance interface{}
}

func (p *internalPluginInstance) Kill()               {}
func (p *internalPluginInstance) Plugin() interface{} { return p.instance }

// externalPluginInstance wraps an external plugin.
type externalPluginInstance struct {
	client   *plugin.Client
	instance interface{}
}

func (p *externalPluginInstance) Kill()               { p.client.Kill() }
func (p *externalPluginInstance) Plugin() interface{} { return p.instance }
