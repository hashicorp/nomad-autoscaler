// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterScaleUtils_IdentifyScaleInNodesHonorsNum(t *testing.T) {
	server := testNomadScaleInServer(t)
	defer server.Close()

	client := testNomadClient(t, server.URL)
	cu := &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
	}

	cfg := map[string]string{
		sdk.TargetConfigKeyClass:             "pool-a",
		sdk.TargetConfigNodeSelectorStrategy: sdk.TargetNodeSelectorStrategyNewestCreateIndex,
	}

	nodes, err := cu.IdentifyScaleInNodes(cfg, 1)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-a", nodes[0].ID)
}

func TestClusterScaleUtils_RunPreScaleInTasksWithRemoteCheckUsesAllEligibleNodes(t *testing.T) {
	server := testNomadScaleInServer(t)
	defer server.Close()

	client := testNomadClient(t, server.URL)
	md := newMockDrainer()
	md.drainerMockFunc = func(_ string, _ *api.DrainOptions, _ *api.WriteOptions) (*api.NodeDrainUpdateResponse, error) {
		return &api.NodeDrainUpdateResponse{EvalCreateIndex: 1}, nil
	}
	md.monitorMockFunc = func(_ context.Context, _ string, _ uint64, _ bool) <-chan *api.MonitorMessage {
		outCh := make(chan *api.MonitorMessage)
		close(outCh)
		return outCh
	}

	cu := &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
		ClusterNodeIDLookupFunc: func(node *api.Node) (string, error) {
			return node.ID, nil
		},
		drainer: md,
	}

	cfg := map[string]string{
		sdk.TargetConfigKeyClass:             "pool-a",
		sdk.TargetConfigNodeSelectorStrategy: sdk.TargetNodeSelectorStrategyNewestCreateIndex,
	}

	ids, err := cu.RunPreScaleInTasksWithRemoteCheck(context.Background(), cfg, []string{"node-b"}, 1)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, NodeResourceID{NomadNodeID: "node-b", RemoteResourceID: "node-b"}, ids[0])
}

func TestClusterScaleUtils_IdentifyScaleInNodesRejectsNonPositiveNum(t *testing.T) {
	server := testNomadScaleInServer(t)
	defer server.Close()

	client := testNomadClient(t, server.URL)
	cu := &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
	}

	cfg := map[string]string{
		sdk.TargetConfigKeyClass:             "pool-a",
		sdk.TargetConfigNodeSelectorStrategy: sdk.TargetNodeSelectorStrategyNewestCreateIndex,
	}

	nodes, err := cu.IdentifyScaleInNodes(cfg, 0)
	require.Error(t, err)
	require.Nil(t, nodes)
	assert.EqualError(t, err, "number of nodes requested for removal must be greater than zero")
}

func TestClusterScaleUtils_RunPreScaleInTasksWithRemoteCheckRejectsNonPositiveNum(t *testing.T) {
	server := testNomadScaleInServer(t)
	defer server.Close()

	client := testNomadClient(t, server.URL)
	cu := &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
		ClusterNodeIDLookupFunc: func(node *api.Node) (string, error) {
			return node.ID, nil
		},
	}

	cfg := map[string]string{
		sdk.TargetConfigKeyClass:             "pool-a",
		sdk.TargetConfigNodeSelectorStrategy: sdk.TargetNodeSelectorStrategyNewestCreateIndex,
	}

	ids, err := cu.RunPreScaleInTasksWithRemoteCheck(context.Background(), cfg, []string{"node-a"}, 0)
	require.Error(t, err)
	require.Nil(t, ids)
	assert.EqualError(t, err, "number of nodes requested for removal must be greater than zero")
}

func testNomadScaleInServer(t *testing.T) *httptest.Server {
	t.Helper()

	nodes := []*api.NodeListStub{
		{
			ID:                    "node-a",
			NodeClass:             "pool-a",
			Status:                api.NodeStatusReady,
			SchedulingEligibility: api.NodeSchedulingEligible,
		},
		{
			ID:                    "node-b",
			NodeClass:             "pool-a",
			Status:                api.NodeStatusReady,
			SchedulingEligibility: api.NodeSchedulingEligible,
		},
	}

	nodeInfo := map[string]*api.Node{
		"node-a": {ID: "node-a"},
		"node-b": {ID: "node-b"},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/nodes":
			require.NoError(t, json.NewEncoder(w).Encode(nodes))
		case "/v1/node/node-a", "/v1/node/node-b":
			n, ok := nodeInfo[r.URL.Path[len("/v1/node/"):]]
			require.True(t, ok)
			require.NoError(t, json.NewEncoder(w).Encode(n))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testNomadClient(t *testing.T, address string) *api.Client {
	t.Helper()

	cfg := api.DefaultConfig()
	cfg.Address = address

	client, err := api.NewClient(cfg)
	require.NoError(t, err)
	return client
}
