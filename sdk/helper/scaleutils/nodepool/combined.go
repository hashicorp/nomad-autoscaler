package nodepool

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
)

const (
	// CombinedClusterPoolIdentifierAnd requires all identifiers to return true.
	CombinedClusterPoolIdentifierAnd CombinedClusterPoolIdentifierMode = "and"

	// CombinedClusterPoolIdentifierOr requires at least one identifier to
	// return true.
	CombinedClusterPoolIdentifierOr CombinedClusterPoolIdentifierMode = "or"
)

// CombinedClusterPoolIdentifierMode defines how different
// ClusterNodePoolIdentifiers are combined.
type CombinedClusterPoolIdentifierMode string

// combinedClusterPoolIdentifier is an implementation of the
// ClusterNodePoolIdentifier interface that filters Nomad nodes by combining
// multiple filters.
type combinedClusterPoolIdentifier struct {
	poolIdentifiers []ClusterNodePoolIdentifier
	mode            CombinedClusterPoolIdentifierMode
}

// NewCombinedClusterPoolIdentifier returns a new combinedClusterPoolIdentifier.
func NewCombinedClusterPoolIdentifier(poolIdentifiers []ClusterNodePoolIdentifier, mode CombinedClusterPoolIdentifierMode) ClusterNodePoolIdentifier {
	return &combinedClusterPoolIdentifier{
		poolIdentifiers: poolIdentifiers,
		mode:            mode,
	}
}

// IsPoolMember satisfies the IsPoolMember function on the
// ClusterNodePoolIdentifier interface.
func (c *combinedClusterPoolIdentifier) IsPoolMember(n *api.NodeListStub) bool {
	// If mode is 'and' we assume the node is a member unless told otherwise.
	isMember := c.mode == CombinedClusterPoolIdentifierAnd

	for _, identifier := range c.poolIdentifiers {
		switch c.mode {
		case CombinedClusterPoolIdentifierAnd:
			isMember = isMember && identifier.IsPoolMember(n)
		case CombinedClusterPoolIdentifierOr:
			isMember = isMember || identifier.IsPoolMember(n)
		}
	}
	return isMember
}

// Key satisfies the Key function on the ClusterNodePoolIdentifier interface.
func (c *combinedClusterPoolIdentifier) Key() string {
	return "combined_identifier"
}

// Value satisfies the Value function on the ClusterNodePoolIdentifier
// interface.
func (c *combinedClusterPoolIdentifier) Value() string {
	values := make([]string, 0, len(c.poolIdentifiers))
	for _, identifier := range c.poolIdentifiers {
		values = append(values, fmt.Sprintf("%s is %s", identifier.Key(), identifier.Value()))
	}
	return strings.Join(values, fmt.Sprintf(" %s ", c.mode))
}
