package policy

import (
	"context"
	"time"
)

// ConfigDefaults holds default configuration for unspecified values.
type ConfigDefaults struct {
	DefaultEvaluationInterval time.Duration
	DefaultCooldown           time.Duration
}

// Source is the interface that must be implemented by backends which provide
// the canonical source for scaling policies.
type Source interface {
	MonitorIDs(ctx context.Context, resultCh chan<- IDMessage, errCh chan<- error)
	MonitorPolicy(ctx context.Context, ID PolicyID, resultCh chan<- Policy, errCh chan<- error)

	// Name returns the SourceName for the implementation. This helps handlers
	// identify the source implementation which is responsible for policies.
	Name() SourceName
}

type PolicyID string

// String satisfies the Stringer interface.
func (p PolicyID) String() string {
	return string(p)
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
)

// IDMessage encapsulates the required information that allows the policy
// manager to launch the correct MonitorPolicy interface function where it
// needs to handle policies which originate from different sources.
type IDMessage struct {
	IDs    []PolicyID
	Source SourceName
}
