// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"errors"
	"testing"

	"github.com/shoenig/test/must"
)

func Test_NewClusterNodePoolIdentifierList(t *testing.T) {
	testCases := []struct {
		inputCfg          map[string]string
		expectedLen       int
		expectedStr       string
		expectedOutputErr error
		name              string
	}{
		{
			inputCfg:          map[string]string{},
			expectedLen:       0,
			expectedOutputErr: errors.New("node pool identification method required"),
			name:              "empty input config",
		},
		{
			inputCfg:          map[string]string{"datacentre": "dc1"},
			expectedLen:       0,
			expectedOutputErr: errors.New("node pool identification method required"),
			name:              "input config with incorrect key",
		},
		{
			inputCfg:    map[string]string{"datacenter": "dc1"},
			expectedLen: 1,
			expectedStr: "datacenter:dc1",
			name:        "datacenter configured in config",
		},
		{
			inputCfg:    map[string]string{"node_class": "high-memory"},
			expectedLen: 1,
			expectedStr: "node_class:high-memory",
			name:        "node_class configured in config",
		},
		{
			inputCfg:    map[string]string{"node_pool": "gpu"},
			expectedLen: 1,
			expectedStr: "node_pool:gpu",
			name:        "node_pool configured in config",
		},
		{
			inputCfg:    map[string]string{"node_class": "high-memory", "datacenter": "dc1"},
			expectedLen: 2,
			expectedStr: "node_class:high-memory and datacenter:dc1",
			name:        "node_class and datacenter are configured in config",
		},
		{
			inputCfg:    map[string]string{"node_pool": "gpu", "datacenter": "dc1"},
			expectedLen: 2,
			expectedStr: "datacenter:dc1 and node_pool:gpu",
			name:        "node_pool and datacenter are configured in config",
		},
		{
			inputCfg: map[string]string{
				"node_class": "high-memory",
				"node_pool":  "gpu",
				"datacenter": "dc1",
			},
			expectedLen: 3,
			expectedStr: "node_class:high-memory and datacenter:dc1 and node_pool:gpu",
			name:        "node_class, node_pool, and datacenter are configured in config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := NewClusterNodePoolIdentifierList(tc.inputCfg)
			if tc.expectedOutputErr != nil {
				must.EqError(t, err, tc.expectedOutputErr.Error())
			} else {
				must.NoError(t, err)
				must.Len(t, tc.expectedLen, ids)
				must.Eq(t, tc.expectedStr, ids.String())
			}
		})
	}
}
