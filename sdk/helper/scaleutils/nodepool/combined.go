// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// ClusterNodePoolIdentifierList is a list of ClusterNodePoolIdentifier
// values that filters nodes using AND logic. A node must match every
// identifier in the list to be considered a pool member.
type ClusterNodePoolIdentifierList []ClusterNodePoolIdentifier

// IsPoolMember returns true if the node matches all identifiers in the list.
func (l ClusterNodePoolIdentifierList) IsPoolMember(n *api.NodeListStub) bool {
	for _, id := range l {
		if !id.IsPoolMember(n) {
			return false
		}
	}
	return true
}

// Encode serializes the identifier list into the combined query format:
// key1=value1+key2=value2. Values are URL-encoded to handle special
// characters (+, =, /).
func (l ClusterNodePoolIdentifierList) Encode() string {
	parts := make([]string, 0, len(l))
	for _, id := range l {
		parts = append(parts, fmt.Sprintf("%s=%s", id.Key(), url.QueryEscape(id.Value())))
	}
	return strings.Join(parts, "+")
}

// DecodeCombinedQueryIdentifiers parses a combined query identifier string
// (key1=value1+key2=value2) and returns the corresponding identifier list.
// Values are URL-decoded to handle special characters.
func DecodeCombinedQueryIdentifiers(encoded string) (ClusterNodePoolIdentifierList, error) {
	pairs := strings.Split(encoded, "+")
	ids := make(ClusterNodePoolIdentifierList, 0, len(pairs))

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
		case sdk.TargetConfigKeyClass:
			ids = append(ids, NewNodeClassPoolIdentifier(value))
		case sdk.TargetConfigKeyDatacenter:
			ids = append(ids, NewNodeDatacenterPoolIdentifier(value))
		case sdk.TargetConfigKeyNodePool:
			ids = append(ids, NewNodePoolClusterPoolIdentifier(value))
		default:
			return nil, fmt.Errorf("unknown pool key %q: must be %s, %s, or %s",
				kv[0], sdk.TargetConfigKeyClass, sdk.TargetConfigKeyDatacenter, sdk.TargetConfigKeyNodePool)
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid identifiers found in combined query")
	}
	return ids, nil
}
