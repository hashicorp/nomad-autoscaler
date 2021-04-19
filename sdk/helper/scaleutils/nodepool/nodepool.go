package nodepool

import (
	"fmt"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// ClusterNodePoolIdentifier is the interface that defines how nodes are
// classed into pools of resources.
type ClusterNodePoolIdentifier interface {

	// IsPoolMember identifies whether the node is a member of the node pool.
	// It is the responsibility of the implementation to read the node info if
	// the required information is not within the stub struct.
	IsPoolMember(*api.NodeListStub) bool

	// Key returns the string representation of the pool identifier.
	Key() string

	// Value returns the pool identifier value that nodes are being filtered
	// by.
	Value() string
}

// NewClusterNodePoolIdentifier generates a new ClusterNodePoolIdentifier based
// on the provided configuration. If a valid option is not found, an error will
// be returned.
func NewClusterNodePoolIdentifier(cfg map[string]string) (ClusterNodePoolIdentifier, error) {

	if class, ok := cfg[sdk.TargetConfigKeyClass]; ok {
		return NewNodeClassPoolIdentifier(class), nil
	}
	if dc, ok := cfg[sdk.TargetConfigKeyDatacenter]; ok {
		return NewNodeDatacenterPoolIdentifier(dc), nil
	}
	return nil, fmt.Errorf("node pool identification method required")
}
