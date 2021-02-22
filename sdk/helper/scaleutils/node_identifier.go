package scaleutils

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
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
			return nil, formattedMultiError(err)
		}

		// Filter out all nodes which do not match the target first.
		if !idFn(node) {
			continue
		}

		// We should class an initializing node as an error, this is caused by
		// node registration and could be sourced from scaling out.
		if node.Status == api.NodeStatusInit {
			err = multierror.Append(err, fmt.Errorf("node %s is initializing", node.ID))
			continue
		}

		// Assuming a cluster has most, if not all nodes in a correct state for
		// scheduling then this is the fastest route. Only append in the event
		// we have not encountered any error to save some cycles.
		if node.SchedulingEligibility == api.NodeSchedulingEligible {
			if err == nil {
				out = append(out, node)
			}
			continue
		}

		// This lifecycle phase relates to nodes that are being drained.
		if node.Drain && node.Status == api.NodeStatusReady {
			err = multierror.Append(err, fmt.Errorf("node %s is draining", node.ID))
			continue
		}

		// This lifecycle phase relates to nodes that typically have had their
		// drain completed, and now await removal from the cluster.
		if !node.Drain && node.Status == api.NodeStatusReady {
			err = multierror.Append(err, fmt.Errorf("node %s is ineligible", node.ID))
		}
	}

	// Be choosy with our returns to avoid sending a large list to the caller
	// that will just get ignored.
	if err != nil {
		return nil, formattedMultiError(err)
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
