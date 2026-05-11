// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"fmt"
	"net/url"
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

// CombinedPoolIdentifier is an extension of ClusterNodePoolIdentifier that
// exposes the sub-identifiers and mode for serialization purposes.
type CombinedPoolIdentifier interface {
	ClusterNodePoolIdentifier

	// Identifiers returns the list of sub-identifiers that compose this
	// combined identifier.
	Identifiers() []ClusterNodePoolIdentifier

	// Mode returns the combination mode (and/or).
	Mode() CombinedClusterPoolIdentifierMode
}

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
		values = append(values, fmt.Sprintf("%s:%s", identifier.Key(), identifier.Value()))
	}
	return strings.Join(values, fmt.Sprintf(" %s ", c.mode))
}

// Identifiers returns the list of sub-identifiers that compose this combined
// identifier.
func (c *combinedClusterPoolIdentifier) Identifiers() []ClusterNodePoolIdentifier {
	return c.poolIdentifiers
}

// Mode returns the combination mode (and/or).
func (c *combinedClusterPoolIdentifier) Mode() CombinedClusterPoolIdentifierMode {
	return c.mode
}

// EncodeCombinedQueryIdentifiers serializes a list of ClusterNodePoolIdentifiers
// into the combined query format: key1=value1+key2=value2
// Values are URL-encoded to handle special characters (+, =, /).
func EncodeCombinedQueryIdentifiers(ids []ClusterNodePoolIdentifier) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%s=%s", id.Key(), url.QueryEscape(id.Value())))
	}
	return strings.Join(parts, "+")
}

// DecodeCombinedQueryIdentifiers parses a combined query identifier string
// (key1=value1+key2=value2) and returns the corresponding ClusterNodePoolIdentifiers.
// Values are URL-decoded to handle special characters.
func DecodeCombinedQueryIdentifiers(encoded string) ([]ClusterNodePoolIdentifier, error) {
	pairs := strings.Split(encoded, "+")
	ids := make([]ClusterNodePoolIdentifier, 0, len(pairs))

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid pool identifier %q: expected key=value", pair)
		}

		value, err := url.QueryUnescape(kv[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode value in pair %q: %v", pair, err)
		}

		switch kv[0] {
		case "node_class", "class":
			ids = append(ids, NewNodeClassPoolIdentifier(value))
		case "datacenter":
			ids = append(ids, NewNodeDatacenterPoolIdentifier(value))
		case "node_pool":
			ids = append(ids, NewNodePoolClusterPoolIdentifier(value))
		default:
			return nil, fmt.Errorf("unknown pool key %q: must be node_class, datacenter, or node_pool", kv[0])
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid identifiers found in combined query")
	}
	return ids, nil
}
