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
	"github.com/shoenig/test/must"
)

func TestClusterScaleUtils_IdentifyScaleInNodes(t *testing.T) {
	testCases := []struct {
		inputNum            int
		expectedNodeCount   int
		expectedFirstNodeID string
		expectedError       error
		name                string
	}{
		{
			inputNum:            1,
			expectedNodeCount:   1,
			expectedFirstNodeID: "node-a",
			expectedError:       nil,
			name:                "honors requested num",
		},
		{
			inputNum:            0,
			expectedNodeCount:   0,
			expectedFirstNodeID: "",
			expectedError:       errInvalidScaleInNum,
			name:                "rejects non positive num",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cu := newTestClusterScaleUtils(t)
			nodes, err := cu.IdentifyScaleInNodes(testScaleInCfg(), tc.inputNum)

			if tc.expectedError != nil {
				must.Error(t, err)
				must.Nil(t, nodes)
				must.ErrorIs(t, err, tc.expectedError)
				return
			}

			must.NoError(t, err)
			must.Len(t, tc.expectedNodeCount, nodes)
			must.Eq(t, tc.expectedFirstNodeID, nodes[0].ID)
		})
	}
}

func TestClusterScaleUtils_RunPreScaleInTasksWithRemoteCheck(t *testing.T) {
	testCases := []struct {
		inputNum              int
		inputRemoteIDs        []string
		expectedResourceID    NodeResourceID
		expectedResourceCount int
		expectedError         error
		name                  string
	}{
		{
			inputNum:              1,
			inputRemoteIDs:        []string{"node-b"},
			expectedResourceID:    NodeResourceID{NomadNodeID: "node-b", RemoteResourceID: "node-b"},
			expectedResourceCount: 1,
			expectedError:         nil,
			name:                  "uses all eligible nodes before remote filtering",
		},
		{
			inputNum:              0,
			inputRemoteIDs:        []string{"node-a"},
			expectedResourceID:    NodeResourceID{},
			expectedResourceCount: 0,
			expectedError:         errInvalidScaleInNum,
			name:                  "rejects non positive num",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cu := newTestClusterScaleUtilsWithRemoteCheck(t)
			ids, err := cu.RunPreScaleInTasksWithRemoteCheck(t.Context(), testScaleInCfg(), tc.inputRemoteIDs, tc.inputNum)

			if tc.expectedError != nil {
				must.Error(t, err)
				must.Nil(t, ids)
				must.ErrorIs(t, err, tc.expectedError)
				return
			}

			must.NoError(t, err)
			must.Len(t, tc.expectedResourceCount, ids)
			must.Eq(t, tc.expectedResourceID, ids[0])
		})
	}
}

func newTestClusterScaleUtils(t *testing.T) *ClusterScaleUtils {
	t.Helper()

	server := testNomadScaleInServer(t)
	t.Cleanup(server.Close)

	client := testNomadClient(t, server.URL)
	return &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
	}
}

func newTestClusterScaleUtilsWithRemoteCheck(t *testing.T) *ClusterScaleUtils {
	t.Helper()

	server := testNomadScaleInServer(t)
	t.Cleanup(server.Close)

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

	return &ClusterScaleUtils{
		log:    hclog.NewNullLogger(),
		client: client,
		ClusterNodeIDLookupFunc: func(node *api.Node) (string, error) {
			return node.ID, nil
		},
		drainer: md,
	}
}

func testScaleInCfg() map[string]string {
	return map[string]string{
		sdk.TargetConfigKeyClass:             "pool-a",
		sdk.TargetConfigNodeSelectorStrategy: sdk.TargetNodeSelectorStrategyNewestCreateIndex,
	}
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
			must.NoError(t, json.NewEncoder(w).Encode(nodes))
		case "/v1/node/node-a", "/v1/node/node-b":
			nID := r.URL.Path[len("/v1/node/"):]
			n, ok := nodeInfo[nID]
			must.True(t, ok, must.Sprintf("unknown node: %q", nID))
			must.NoError(t, json.NewEncoder(w).Encode(n))
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
	must.NoError(t, err)
	return client
}
