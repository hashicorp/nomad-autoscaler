// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// newestCreateIndexClusterScaleInNodeSelector is the NodeSelector
// implementation of the ClusterScaleInNodeSelector interface. It selects nodes
// based off the list ordering from Nomad and is the default selector.
type newestCreateIndexClusterScaleInNodeSelector struct{}

// newNewestCreateIndexClusterScaleInNodeSelector returns a new
// newestCreateIndexClusterScaleInNodeSelector implementation of the
// ClusterScaleInNodeSelector interface.
func newNewestCreateIndexClusterScaleInNodeSelector() ClusterScaleInNodeSelector {
	return &newestCreateIndexClusterScaleInNodeSelector{}
}

// Name satisfies the Name function on the ClusterScaleInNodeSelector
// interface.
func (n *newestCreateIndexClusterScaleInNodeSelector) Name() string {
	return sdk.TargetNodeSelectorStrategyNewestCreateIndex
}

// Select satisfies the Select function on the ClusterScaleInNodeSelector
// interface.
func (n *newestCreateIndexClusterScaleInNodeSelector) Select(nodes []*api.NodeListStub, num int) []*api.NodeListStub {

	if len(nodes) < num {
		num = len(nodes)
	}
	out := make([]*api.NodeListStub, num)

	for i := 0; i <= num-1; i++ {
		out[i] = nodes[i]
	}
	return out
}
