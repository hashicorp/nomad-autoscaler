// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// DefaultQueryWindow is the value used if `query_window` is not specified in
// a policy check.
const DefaultQueryWindow = time.Minute

// PolicyID contains identifying information about a policy, as returned by
// policy.Source.MonitorIDs()
type PolicyID = string

// ConfigDefaults holds default configuration for unspecified values.
type ConfigDefaults struct {
	DefaultEvaluationInterval time.Duration
	DefaultCooldown           time.Duration
}

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
	// MonitorIDs will return a list of all existing policies every time there
	// is a change in one of them, but each policy will have a flag indicating
	// if it was the one that generated the update.
	MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq)

	GetLatestVersion(ctx context.Context, pID PolicyID) (*sdk.ScalingPolicy, error)

	// Name returns the SourceName for the implementation. This helps handlers
	// identify the source implementation which is responsible for policies.
	Name() SourceName

	// ReloadIDsMonitor is used to trigger a reload of the MonitorIDs routine
	// so that config items can be reloaded gracefully without restarting the
	// agent.
	ReloadIDsMonitor()
	MonitorPolicy(ctx context.Context, monitorPolicyReq MonitorPolicyReq)
}

// SourceName differentiates policies from different sources. This allows the
// policy manager to use the correct Source interface implementation to launch
// the MonitorPolicy function for the PolicyID.
type SourceName string

const (
	// SourceNameNomad is the source for policies that originate from the Nomad
	// scaling policies API.
	SourceNameNomad SourceName = "nomad"

	// SourceNameFile is the source for policies that are loaded from disk.
	SourceNameFile SourceName = "file"

	// SourceNameHA is the source for HA policy sources
	SourceNameHA SourceName = "ha"
)

// HandleSourceError provides common functionality when a policy source
// encounters an ephemeral or non-critical error.
func HandleSourceError(name SourceName, err error, errCha chan<- error) {

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
// It contains a map of PolicyID to a boolean indicating whether the policy was
// recently updated.
type IDMessage struct {
	IDs    map[PolicyID]bool
	Source SourceName
}
