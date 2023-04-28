// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// ClusterScaleInNodeSelector is the interface that defines how nodes are
// selected for termination when performing scale in actions.
type ClusterScaleInNodeSelector interface {

	// Name returns the name of the node selector strategy.
	Name() string

	// Select is used to filter the passed node list, and select an appropriate
	// number of nodes which fit the criteria of the strategy. The return list
	// is not guaranteed to match the required number, nor include any items if
	// selection is impossible. The function will never return more nodes than
	// asked for.
	Select([]*api.NodeListStub, int) []*api.NodeListStub
}

// NewSelector takes the user configuration and creates a
// ClusterScaleInNodeSelector for use. In the event the configuration cannot be
// understood, an error will be returned.
func NewSelector(cfg map[string]string, client *api.Client, log hclog.Logger) (ClusterScaleInNodeSelector, error) {

	// If the user has not configured the strategy, set the default.
	val, ok := cfg[sdk.TargetConfigNodeSelectorStrategy]
	if !ok {
		val = sdk.TargetNodeSelectorStrategyLeastBusy
	}

	// If the user configured a value but we are unable to understand this, an
	// error is returned rather than defaulting so we do not terminate nodes
	// that the user wasn't expecting.
	switch val {
	case sdk.TargetNodeSelectorStrategyLeastBusy:
		return newLeastBusyClusterScaleInNodeSelector(client, log), nil
	case sdk.TargetNodeSelectorStrategyNewestCreateIndex:
		return newNewestCreateIndexClusterScaleInNodeSelector(), nil
	case sdk.TargetNodeSelectorStrategyEmpty:
		return newEmptyClusterScaleInNodeSelector(client, log, false), nil
	case sdk.TargetNodeSelectorStrategyEmptyIgnoreSystemJobs:
		return newEmptyClusterScaleInNodeSelector(client, log, true), nil
	default:
		return nil, fmt.Errorf("unsupported node selector strategy: %v", val)
	}
}
