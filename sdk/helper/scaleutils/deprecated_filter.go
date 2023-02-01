// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
)

// Deprecated. Please use NodeResourceID.
//
// NodeID provides a mapping between the Nomad ID of a node and its remote
// infrastructure provider specific ID.
type NodeID struct {
	NomadID, RemoteID string
}

// Deprecated. Please use nodepool.ClusterNodePoolIdentifier.
//
// PoolIdentifier is the information used to identify nodes into pools of
// resources. This then forms our scalable unit.
type PoolIdentifier struct {
	IdentifierKey IdentifierKey
	Value         string
}

// Validate is used to check the validation of the PoolIdentifier object.
func (p *PoolIdentifier) Validate() error {
	switch p.IdentifierKey {
	case IdentifierKeyClass:
		return nil
	default:
		return fmt.Errorf("unsupported node pool identifier: %q", p.IdentifierKey)
	}
}

// IdentifyNodes filters the supplied node list based on the PoolIdentifier
// params.
func (p *PoolIdentifier) IdentifyNodes(n []*api.NodeListStub) ([]*api.NodeListStub, error) {
	switch p.IdentifierKey {
	case IdentifierKeyClass:
		return filterByClass(n, p.Value)
	default:
		return nil, fmt.Errorf("unsupported node pool identifier: %q", p.IdentifierKey)
	}
}

// Deprecated. Please use nodepool.ClusterNodePoolIdentifier.
//
// IdentifierKey is the identifier to group nodes into a pool of resource and
// thus forms the scalable object.
type IdentifierKey string

// IdentifierKeyClass uses the Node.Class field to identify nodes into pools of
// resource. This is the default.
const IdentifierKeyClass IdentifierKey = "class"

// Deprecated. Pleas use ClusterNodeIDLookupFunc.
//
// RemoteProvider is infrastructure provider which hosts and therefore manages
// the Nomad client instances. This is used to understand how to translate the
// Nomad NodeID to an ID that the provider understands.
type RemoteProvider string

// RemoteProviderAWSInstanceID is the Amazon Web Services remote provider for
// EC2 instances. This provider will use the node attribute as defined by
// nodeAttrAWSInstanceID to perform ID translation.
const RemoteProviderAWSInstanceID RemoteProvider = "aws_instance_id"

// RemoteProviderAzureInstanceID is the Azure remote provider for
// VM instances. This provider will use the node attribute as defined by
// nodeAttrAzureInstanceID to perform ID translation.
const RemoteProviderAzureInstanceID RemoteProvider = "azure_instance_id"

const RemoteProviderGCEInstanceID RemoteProvider = "gce_instance_id"

// Deprecated. Please use ClusterScaleInNodeIDStrategy.
//
// NodeIDStrategy is the strategy used to identify nodes for removal as part of
// scaling in.
type NodeIDStrategy string

// IDStrategyNewestCreateIndex uses the Nomad Nodes().List() output in the
// order it is presented. This means we do not need additional sorting and thus
// it is fastest. In an environment that uses bin-packing this may also be
// preferable as nodes with older create indexes are expected to be most
// packed.
const IDStrategyNewestCreateIndex NodeIDStrategy = "newest_create_index"

// nodeAttrAWSInstanceID is the node attribute to use when identifying the
// AWS instanceID of a node.
const nodeAttrAWSInstanceID = "unique.platform.aws.instance-id"

// nodeAttrAzureName is the node attribute to use when identifying the
// Azure instanceID of a node.
const nodeAttrAzureInstanceID = "unique.platform.azure.name"

// nodeAttrGCEHostname is the node attribute to use when identifying the
// GCE hostname of a node.
const nodeAttrGCEHostname = "unique.platform.gce.hostname"

// nodeAttrGCEZone is the node attribute to use when identifying the GCE
// zonde of a node.
const nodeAttrGCEZone = "platform.gce.zone"

// defaultClassIdentifier is the class used for nodes which have an empty class
// parameter when using the IdentifierKeyClass.
const defaultClassIdentifier = "autoscaler-default-pool"

// filterByClass returns a filtered list of nodes which are active in the
// cluster and where the specified class matches that of the nodes. In the
// event that nodes are found within the class pool in an unstable state, and
// thus indicating there is change occurring; an error will be returned.
func filterByClass(n []*api.NodeListStub, id string) ([]*api.NodeListStub, error) {

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
			err.ErrorFormat = errHelper.MultiErrorFunc
			return nil, err
		}

		// Filter out all nodes which do not match the target class first.
		if node.NodeClass != "" && node.NodeClass != id ||
			node.NodeClass == "" && id != defaultClassIdentifier {
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
		// drain completed, and now await removal from the cluster. Beyond this
		// point nodes are considered down and waiting for GC from the cluster,
		// therefore they are safe to ignore and do not impact our
		// classification of pool stability.
		if !node.Drain && node.Status == api.NodeStatusReady {
			err = multierror.Append(err, fmt.Errorf("node %s is ineligible", node.ID))
		}
	}

	// Be choosy with our returns to avoid sending a large list to the caller
	// that will just get ignored.
	if err != nil {
		err.ErrorFormat = errHelper.MultiErrorFunc
		return nil, err
	}
	return out, nil
}

// nodeIDMapFunc is the function signature used to find the Nomad node's remote
// identifier. Specific implementations can be found below.
type nodeIDMapFunc func(n *api.Node) (string, error)

// idFuncMap contains a mapping of RemoteProvider to the function which can
// pull the remote ID information from the node.
var idFuncMap = map[RemoteProvider]nodeIDMapFunc{
	RemoteProviderAWSInstanceID:   awsNodeIDMap,
	RemoteProviderAzureInstanceID: azureNodeIDMap,
	RemoteProviderGCEInstanceID:   gceNodeIDMap,
}

// awsNodeIDMap is used to identify the AWS InstanceID of a Nomad node using
// the relevant attribute value.
func awsNodeIDMap(n *api.Node) (string, error) {
	var err error
	val, ok := n.Attributes[nodeAttrAWSInstanceID]
	if !ok {
		err = fmt.Errorf("attribute %q not found", nodeAttrAWSInstanceID)
	}
	return val, err
}

// azureNodeIDMap is used to identify the Azure InstanceID of a Nomad node using
// the relevant attribute value.
func azureNodeIDMap(n *api.Node) (string, error) {
	if val, ok := n.Attributes[nodeAttrAzureInstanceID]; ok {
		return val, nil
	}

	// Fallback to meta tag.
	if val, ok := n.Meta[nodeAttrAzureInstanceID]; ok {
		return val, nil
	}

	return "", fmt.Errorf("attribute %q not found", nodeAttrAzureInstanceID)
}

// gceNodeIDMap is used to identify the GCE Instance of a Nomad node using
// the relevant attribute value.
func gceNodeIDMap(n *api.Node) (string, error) {
	zone, ok := n.Attributes[nodeAttrGCEZone]
	if !ok {
		return "", fmt.Errorf("attribute %q not found", nodeAttrGCEZone)
	}
	hostname, ok := n.Attributes[nodeAttrGCEHostname]
	if !ok {
		return "", fmt.Errorf("attribute %q not found", nodeAttrGCEHostname)
	}
	if idx := strings.Index(hostname, "."); idx != -1 {
		return fmt.Sprintf("zones/%s/instances/%s", zone, hostname[0:idx]), nil
	} else {
		return fmt.Sprintf("zones/%s/instances/%s", zone, hostname), nil
	}
}
