package scaleutils

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// NodeIDMap provides a mapping between the Nomad ID of a node and its remote
// infrastructure provider specific ID.
type NodeIDMap struct {
	NomadID, RemoteID string
}

// IdentifierKey is the identifier to group nodes into a pool of resource and
// thus forms the scalable object.
type IdentifierKey string

// IdentifierKeyClass uses the Node.Class field to identify nodes into pools of
// resource. This is the default.
const IdentifierKeyClass IdentifierKey = "class"

// RemoteProvider is infrastructure provider which hosts and therefore manages
// the Nomad client instances. This is used to understand how to translate the
// Nomad NodeID to an ID that the provider understands.
type RemoteProvider string

// RemoteProviderAWS is the Amazon Web Services remote provider. This provider
// will use the node attribute as defined by nodeAttrAWSInstanceID to perform
// ID translation.
const RemoteProviderAWS RemoteProvider = "aws"

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

// defaultClassIdentifier is the class used for nodes which have an empty class
// parameter when using the IdentifierKeyClass.
const defaultClassIdentifier = "autoscaler-default-pool"

// filterByClass returns a filtered list of nodes which are active in the
// cluster and where the specified class matches that of the nodes.
func filterByClass(n []*api.NodeListStub, id string) []*api.NodeListStub {

	// Create our output list object.
	var out []*api.NodeListStub

	for _, node := range n {

		// Ignore nodes that are not in a ready state.
		if node.Status != api.NodeStatusReady {
			continue
		}

		// Ignore nodes that are not currently eligible.
		if node.SchedulingEligibility != api.NodeSchedulingEligible {
			continue
		}

		if node.Drain {
			continue
		}

		// Perform the node class check. We ensure an empty class is treated as
		// the default value.
		if node.NodeClass != "" && node.NodeClass == id ||
			node.NodeClass == "" && id == defaultClassIdentifier {
			out = append(out, node)
		}
	}

	return out
}

// nodeIDMapFunc is the function signature used to find the Nomad nodes remote
// identifier. Specific implementations can be found below.
type nodeIDMapFunc func(n *api.Node) (string, error)

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
