// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package agent

import "context"

func (a *Agent) initEnt(_ context.Context) {}
