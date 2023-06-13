// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type Name string

const (
	// NameNomad is the source for policies that originate from the Nomad
	// scaling policies API.
	NameNomad Name = "nomad"

	// NameFile is the source for policies that are loaded from disk.
	NameFile Name = "file"

	// NameHA is the source for HA policy sources
	NameHA Name = "ha"
)

type MonitorIDsReq struct {
	ErrCh    chan<- error
	ResultCh chan<- IDMessage
}

type MonitorPolicyReq struct {
	ID       PolicyID
	ErrCh    chan<- error
	ReloadCh <-chan struct{}
	ResultCh chan<- sdk.ScalingPolicy
}

// Source is the interface that must be implemented by backends which provide
// the canonical source for scaling policies.
type Source interface {
	MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq)
	MonitorPolicy(ctx context.Context, monitorPolicyReq MonitorPolicyReq)

	// Name returns the SourceName for the implementation. This helps handlers
	// identify the source implementation which is responsible for policies.
	Name() Name

	// ReloadIDsMonitor is used to trigger a reload of the MonitorIDs routine
	// so that config items can be reloaded gracefully without restarting the
	// agent.
	ReloadIDsMonitor()
}

// PolicyID contains identifying information about a policy, as returned by
// policy.Source.MonitorIDs()
type PolicyID string

// String satisfies the Stringer interface.
func (p PolicyID) String() string {
	return string(p)
}

// SourceName differentiates policies from different sources. This allows the
// policy manager to use the correct Source interface implementation to launch
// the MonitorPolicy function for the PolicyID.

// HandleSourceError provides common functionality when a policy source
// encounters an ephemeral or non-critical error.
func HandleSourceError(name Name, err error, errCha chan<- error) {

	// Emit our metric so errors can be visualised and alerted on from APM
	// tools. This helps operators identify potential issues in communicating
	// with a policy source.
	metrics.IncrCounterWithLabels(
		[]string{"policy", "source", "error_count"},
		1,
		[]metrics.Label{{Name: "policy_source", Value: string(name)}})

	// Send the error to the channel for the handler/manager can perform the
	// work it needs to.
	errCha <- err
}

// IDMessage encapsulates the required information that allows the policy
// manager to launch the correct MonitorPolicy interface function where it
// needs to handle policies which originate from different sources.
type IDMessage struct {
	IDs    []PolicyID
	Source Name
}
