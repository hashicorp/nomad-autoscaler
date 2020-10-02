package scaleutils

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// NodeID provides a mapping between the Nomad ID of a node and its remote
// infrastructure provider specific ID.
type NodeID struct {
	NomadID, RemoteID string
}

// PoolIdentifier is the information used to identify nodes into pools of
// resources. This then forms our scalable unit.
type PoolIdentifier struct {
	IdentifierKey IdentifierKey
	Value         string
}

// IdentifyNodes filters the supplied node list based on the PoolIdentifier
// params.
func (p *PoolIdentifier) IdentifyNodes(n []*api.NodeListStub) ([]*api.NodeListStub, error) {
	switch p.IdentifierKey {
	case IdentifierKeyClass:
		return filterByClass(n, p.Value), nil
	default:
		return nil, fmt.Errorf("unsupported node pool identifier: %q", p.IdentifierKey)
	}
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

// RemoteProviderAWSInstanceID is the Amazon Web Services remote provider for
// EC2 instances. This provider will use the node attribute as defined by
// nodeAttrAWSInstanceID to perform ID translation.
const RemoteProviderAWSInstanceID RemoteProvider = "aws_instance_id"

// RemoteProviderAzureInstanceID is the Azure remote provider for
// VM instances. This provider will use the node attribute as defined by
// nodeAttrAzureInstanceID to perform ID translation.
const RemoteProviderAzureInstanceID RemoteProvider = "azure_instance_id"

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

// nodeIDMapFunc is the function signature used to find the Nomad node's remote
// identifier. Specific implementations can be found below.
type nodeIDMapFunc func(n *api.Node) (string, error)

// idFuncMap contains a mapping of RemoteProvider to the function which can
// pull the remote ID information from the node.
var idFuncMap = map[RemoteProvider]nodeIDMapFunc{
	RemoteProviderAWSInstanceID:   awsNodeIDMap,
	RemoteProviderAzureInstanceID: azureNodeIDMap,
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
	var err error
	val, ok := n.Attributes[nodeAttrAzureInstanceID]
	if !ok {
		err = fmt.Errorf("attribute %q not found", nodeAttrAzureInstanceID)
	}
	return val, err
}
