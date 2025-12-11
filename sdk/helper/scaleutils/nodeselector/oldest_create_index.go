// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// oldestCreateIndexClusterScaleInNodeSelector is the NodeSelector
// implementation of the ClusterScaleInNodeSelector interface. It selects nodes
// based off the list ordering from Nomad and is the default selector.
type oldestCreateIndexClusterScaleInNodeSelector struct{}

// newoldestCreateIndexClusterScaleInNodeSelector returns a new
// oldestCreateIndexClusterScaleInNodeSelector implementation of the
// ClusterScaleInNodeSelector interface.
func newOldestCreateIndexClusterScaleInNodeSelector() ClusterScaleInNodeSelector {
	return &oldestCreateIndexClusterScaleInNodeSelector{}
}

// Name satisfies the Name function on the ClusterScaleInNodeSelector
// interface.
func (n *oldestCreateIndexClusterScaleInNodeSelector) Name() string {
	return sdk.TargetNodeSelectorStrategyOldestCreateIndex
}

// Select satisfies the Select function on the ClusterScaleInNodeSelector
// interface.
func (n *oldestCreateIndexClusterScaleInNodeSelector) Select(nodes []*api.NodeListStub, num int) []*api.NodeListStub {

	last_index := len(nodes) - 1
	if len(nodes) < num {
		num = len(nodes)
	}
	out := make([]*api.NodeListStub, num)

	for i := num - 1; i >= 0; i-- {
		out[i] = nodes[(last_index)-i]
	}
	return out
}
