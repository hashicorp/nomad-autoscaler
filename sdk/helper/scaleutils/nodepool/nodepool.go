// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

const (
	defaultignoreDrainingNodes = false
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
	class, hasClass := cfg[sdk.TargetConfigKeyClass]
	dc, hasDC := cfg[sdk.TargetConfigKeyDatacenter]

	ignoreDrainingNodes := defaultignoreDrainingNodes

	if ignoreDrainingNodesString, ok := cfg[sdk.TargetIgnoreDrainingNodes]; ok {
		idn, err := strconv.ParseBool(ignoreDrainingNodesString)
		if err != nil {
			return nil, fmt.Errorf("node pool identification method required")
		} else {
			ignoreDrainingNodes = idn
		}
	}

	switch {
	case hasClass && hasDC:
		return NewCombinedClusterPoolIdentifier(
			[]ClusterNodePoolIdentifier{
				NewNodeClassPoolIdentifier(class, ignoreDrainingNodes),
				NewNodeDatacenterPoolIdentifier(dc, ignoreDrainingNodes),
			},
			CombinedClusterPoolIdentifierAnd,
		), nil
	case hasClass:
		return NewNodeClassPoolIdentifier(class, ignoreDrainingNodes), nil
	case hasDC:
		return NewNodeDatacenterPoolIdentifier(dc, ignoreDrainingNodes), nil
	}

	return nil, fmt.Errorf("node pool identification method required")
}
