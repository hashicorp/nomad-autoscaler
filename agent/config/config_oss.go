// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package config

// DefaultEntConfig allows configuring enterprise only default configuration
// values.
func DefaultEntConfig() *Agent { return &Agent{} }
