// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func NodeAllocsTestServer(t *testing.T, cluster string) (*httptest.Server, func()) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Extract node ID from path.
		path := strings.TrimPrefix(r.URL.Path, "/v1/node/")
		nodeID := strings.TrimSuffix(path, "/allocations")

		// Read file from test-fixtures/empty folder.
		fileName := fmt.Sprintf("test-fixtures/%s/%s.json", cluster, nodeID)
		nodeStatus, err := os.ReadFile(fileName)
		if err != nil {
			t.Errorf("failed to read file: %v", err)
		}

		fmt.Fprint(w, string(nodeStatus))
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	return httptest.NewServer(http.HandlerFunc(handler)), func() { ts.Close() }
}
