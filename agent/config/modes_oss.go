// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package config

// ModesEnabled lists the modes that are allowed to be used.
// With !ent build no special modes are allowed.
var ModesEnabled = []string{}
