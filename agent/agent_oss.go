// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package agent

import (
	"context"
)

func (a *Agent) initEnt(ctx context.Context, reload <-chan any) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reload:
				continue
			}
		}
	}()
}
