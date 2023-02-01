// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sdk

const (
	// PluginTypeBase is plugin component that all plugins must implement.
	PluginTypeBase = "base"

	// PluginTypeAPM is a plugin which satisfies the APM interface.
	PluginTypeAPM = "apm"

	// PluginTypeTarget is a plugin which satisfies the Target interface.
	PluginTypeTarget = "target"

	// PluginTypeStrategy is a plugin which satisfies the Strategy interface.
	PluginTypeStrategy = "strategy"
)
