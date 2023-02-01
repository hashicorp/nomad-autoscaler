// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package manager

import "github.com/hashicorp/nomad-autoscaler/agent/config"

func isEnterprise(_ string) bool                                          { return false }
func (pm *PluginManager) loadEnterprisePlugin(_ *config.Plugin, _ string) {}
