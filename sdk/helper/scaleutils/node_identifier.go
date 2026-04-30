// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"fmt"
	"strconv"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
)

// NodeResourceID maps a Nomad node ID to a remote resource ID.
type NodeResourceID struct {

	// NomadNodeID is the ID as seen within the Nomad api.Node object.
	NomadNodeID string

	// RemoteResourceID is the remote resource ID of the server/instance such
	// as the AWS EC2 instance ID.
	RemoteResourceID string
}

// ClusterNodeIDLookupFunc is the callback function signature used to identify
// a nodes remote ID from the api.Node object. This allows the function to be
// defined by external plugins.
type ClusterNodeIDLookupFunc func(*api.Node) (string, error)

// ClusterScaleInNodeIDStrategy identifies the method in which nodes are
// selected from the node pool for removal during scale in actions.
type ClusterScaleInNodeIDStrategy string

const (
	// NewestCreateIndexClusterScaleInNodeIDStrategy uses the Nomad
	// Nodes().List() output in the order it is presented. This means we do not
	// need additional sorting and thus it is fastest. In an environment that
	// uses bin-packing this may also be preferable as nodes with older create
	// indexes are expected to be most packed.
	NewestCreateIndexClusterScaleInNodeIDStrategy ClusterScaleInNodeIDStrategy = "newest_create_index"
)

// FilterNodes returns a filtered list of nodes which are active in the cluster
// and where they pass the match performed by the idFn. In the event that nodes
// are found within the pool in an unstable state, and thus indicating there is
// change occurring; an error will be returned.
func FilterNodes(n []*api.NodeListStub, idFn func(*api.NodeListStub) bool) ([]*api.NodeListStub, error) {
	return FilterNodesWithOptions(n, idFn, nil)
}

// FilterNodesWithOptions returns a filtered list of nodes using additional
// filtering options for handling initializing and draining nodes.
//
// This function is supported with caveats. Ignoring initializing or draining
// nodes can improve responsiveness during long transitions, but may increase
// the risk of oscillation or temporary capacity overshoot.
func FilterNodesWithOptions(n []*api.NodeListStub, idFn func(*api.NodeListStub) bool, opts *NodeFilterOptions) ([]*api.NodeListStub, error) {

	// Create our output list object.
	var out []*api.NodeListStub

	// Track upto 10 nodes which are deemed to cause an error to the
	// autoscaler. It is possible to make an argument that the first error
	// should be returned in order to improve speed. In a situation where two
	// nodes are in an undesired state, it would require the operator to
	// perform the same tidy and restart of the autoscaler loop twice, which
	// seems worse than having some extra time within this function.
	var err *multierror.Error

	for _, node := range n {

		// Track till 10 nodes in an unstable state so that we have some
		// efficiency, whilst still responding with useful information. It also
		// avoids error logs messages which are extremely long and potentially
		// unsuitable for log aggregators.
		if err != nil && err.Len() >= 10 {
			return nil, errHelper.FormattedMultiError(err)
		}

		// Filter out nodes which do not match the policy filter criteria first.
		if !idFn(node) {
			continue
		}

		// Assuming a cluster has most, if not all nodes in a correct state for
		// scheduling then this is the fastest route. Only append in the event
		// we have not encountered any error to save some cycles.
		//
		// Ineligible nodes are excluded from pool capacity by default. When
		// IgnoreIneligible is set (e.g., scale-in), include them so they
		// remain termination candidates.
		if !node.Drain && node.Status == api.NodeStatusReady {
			if node.SchedulingEligibility == api.NodeSchedulingEligible || opts.IgnoreIneligible() {
				if err == nil {
					out = append(out, node)
				}
			}
			continue
		}

		// We should class an initializing node as an error, this is caused by
		// node registration and could be sourced from scaling out.
		if node.Status == api.NodeStatusInit && !opts.IgnoreInit() {
			err = multierror.Append(err, fmt.Errorf("node %s is initializing", node.ID))
			continue
		}

		// Avoid scaling the cluster if there are nodes being drained since the
		// cluster may be in an unstable state. If a scaling action is still
		// required it will happen in the next policy evaluation.
		if node.Drain && !opts.IgnoreDrain() {
			err = multierror.Append(err, fmt.Errorf("node %s is draining", node.ID))
			continue
		}
	}

	// Be choosy with our returns to avoid sending a large list to the caller
	// that will just get ignored.
	if err != nil {
		return nil, errHelper.FormattedMultiError(err)
	}
	return out, nil
}

// filterOutNodeID removes the node as specified by the ID function parameter
// from the list of nodes if it is found.
func filterOutNodeID(n []*api.NodeListStub, id string) []*api.NodeListStub {

	// Protect against instances where the ID is empty.
	if id == "" {
		return n
	}

	// Iterate the node list, removing and returning the modified list if an ID
	// match is found.
	for i, node := range n {
		if node.ID == id {
			n[len(n)-1], n[i] = n[i], n[len(n)-1]
			return n[:len(n)-1]
		}
	}
	return n
}

// Node filter options are supported with caveats.
//
// Enabling ignore options can reduce blocked evaluations in environments with
// long drain/init windows, but may reduce strict readiness guarantees.
const (
	XNodeFilterOptionIgnoreInit       = "node_filter_ignore_init"
	XNodeFilterOptionIgnoreDrain      = "node_filter_ignore_drain"
	XNodeFilterOptionIgnoreIneligible = "node_filter_ignore_ineligible"
)

type NodeFilterOptions struct {
	ignoreInit       bool
	ignoreDrain      bool
	ignoreIneligible bool
}

func NewNodeFilterOptions(cfg map[string]string) (*NodeFilterOptions, error) {
	opts := &NodeFilterOptions{}

	if str, ok := cfg[XNodeFilterOptionIgnoreInit]; ok {
		ignoreInit, err := strconv.ParseBool(str)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse value %s for %s configuration: %v",
				str, XNodeFilterOptionIgnoreInit, err,
			)
		}

		opts.ignoreInit = ignoreInit
	}

	if str, ok := cfg[XNodeFilterOptionIgnoreDrain]; ok {
		ignoreDrain, err := strconv.ParseBool(str)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse value %s for %s configuration: %v",
				str, XNodeFilterOptionIgnoreDrain, err,
			)
		}

		opts.ignoreDrain = ignoreDrain
	}

	if str, ok := cfg[XNodeFilterOptionIgnoreIneligible]; ok {
		ignoreIneligible, err := strconv.ParseBool(str)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse value %s for %s configuration: %v",
				str, XNodeFilterOptionIgnoreIneligible, err,
			)
		}

		opts.ignoreIneligible = ignoreIneligible
	}

	return opts, nil
}

func (n *NodeFilterOptions) IgnoreInit() bool {
	if n == nil {
		return false
	}
	return n.ignoreInit
}

func (n *NodeFilterOptions) IgnoreDrain() bool {
	if n == nil {
		return false
	}
	return n.ignoreDrain
}

func (n *NodeFilterOptions) IgnoreIneligible() bool {
	if n == nil {
		return false
	}
	return n.ignoreIneligible
}
