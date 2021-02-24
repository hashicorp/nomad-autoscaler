package nodepool

import "github.com/hashicorp/nomad/api"

// defaultClassIdentifier is the class value used when the nodes NodeClass is
// empty.
const defaultClassIdentifier = "autoscaler-default-pool"

// nodeClassClusterPoolIdentifier is the NodeClass implementation of the
// ClusterNodePoolIdentifier interface and filters Nomad nodes by their
// Node.NodeClass parameter.
type nodeClassClusterPoolIdentifier struct {
	id string
}

// NewNodeClassPoolIdentifier returns a new nodeClassClusterPoolIdentifier
// implementation of the ClusterNodePoolIdentifier interface.
func NewNodeClassPoolIdentifier(id string) ClusterNodePoolIdentifier {
	return &nodeClassClusterPoolIdentifier{
		id: id,
	}
}

// NodeIsPoolMember satisfies the NodeIsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (n nodeClassClusterPoolIdentifier) IsPoolMember(node *api.NodeListStub) bool {
	return node.NodeClass != "" && node.NodeClass == n.id ||
		node.NodeClass == "" && n.id == defaultClassIdentifier
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (n nodeClassClusterPoolIdentifier) Key() string { return "node_class" }

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (n nodeClassClusterPoolIdentifier) Value() string { return n.id }
