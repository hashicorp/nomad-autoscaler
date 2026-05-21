// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package nodepool

import (
	"errors"
	"testing"

	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

func Test_NewClusterNodePoolIdentifierList(t *testing.T) {
	testCases := []struct {
		inputCfg          map[string]string
		expectedOutputLen int
		expectedOutputErr error
		name              string
	}{
		{
			inputCfg:          map[string]string{},
			expectedOutputLen: 0,
			expectedOutputErr: errors.New("node pool identification method required"),
			name:              "empty input config",
		},
		{
			inputCfg:          map[string]string{"datacentre": "dc1"},
			expectedOutputLen: 0,
			expectedOutputErr: errors.New("node pool identification method required"),
			name:              "input config with incorrect key",
		},
		{
			inputCfg:          map[string]string{"datacenter": "dc1"},
			expectedOutputLen: 1,
			expectedOutputErr: nil,
			name:              "datacenter configured in config",
		},
		{
			inputCfg:          map[string]string{"node_class": "high-memory"},
			expectedOutputLen: 1,
			expectedOutputErr: nil,
			name:              "node_class configured in config",
		},
		{
			inputCfg:          map[string]string{"node_pool": "gpu"},
			expectedOutputLen: 1,
			expectedOutputErr: nil,
			name:              "node_pool configured in config",
		},
		{
			inputCfg:          map[string]string{"node_class": "high-memory", "datacenter": "dc1"},
			expectedOutputLen: 2,
			expectedOutputErr: nil,
			name:              "node_class and datacenter are configured in config",
		},
		{
			inputCfg:          map[string]string{"node_pool": "gpu", "datacenter": "dc1"},
			expectedOutputLen: 2,
			expectedOutputErr: nil,
			name:              "node_pool and datacenter are configured in config",
		},
		{
			inputCfg: map[string]string{
				"node_class": "high-memory",
				"node_pool":  "gpu",
				"datacenter": "dc1",
			},
			expectedOutputLen: 3,
			expectedOutputErr: nil,
			name:              "node_class, node_pool, and datacenter are configured in config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := NewClusterNodePoolIdentifierList(tc.inputCfg)
			if tc.expectedOutputErr != nil {
				assert.Equal(t, tc.expectedOutputErr, err, tc.name)
				assert.Nil(t, ids, tc.name)
			} else {
				assert.NoError(t, err, tc.name)
				assert.Len(t, ids, tc.expectedOutputLen, tc.name)
			}
		})
	}
}

func Test_NewClusterNodePoolIdentifier(t *testing.T) {
	testCases := []struct {
		inputCfg       map[string]string
		expectedKey    string
		expectedValue  string
		expectedErrMsg string
		name           string
	}{
		{
			inputCfg:       map[string]string{},
			expectedErrMsg: "node pool identification method required",
			name:           "empty config returns error",
		},
		{
			inputCfg:      map[string]string{"node_class": "high-memory"},
			expectedKey:   "node_class",
			expectedValue: "high-memory",
			name:          "single node_class returns class identifier",
		},
		{
			inputCfg:      map[string]string{"datacenter": "dc1"},
			expectedKey:   "datacenter",
			expectedValue: "dc1",
			name:          "single datacenter returns dc identifier",
		},
		{
			inputCfg:      map[string]string{"node_pool": "gpu"},
			expectedKey:   "node_pool",
			expectedValue: "gpu",
			name:          "single node_pool returns pool identifier",
		},
		{
			inputCfg:      map[string]string{"node_class": "high-memory", "datacenter": "dc1"},
			expectedKey:   "node_class",
			expectedValue: "high-memory",
			name:          "multiple keys returns first (node_class takes priority)",
		},
		{
			inputCfg:      map[string]string{"datacenter": "dc1", "node_pool": "gpu"},
			expectedKey:   "datacenter",
			expectedValue: "dc1",
			name:          "datacenter and node_pool returns datacenter (priority order)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := NewClusterNodePoolIdentifier(tc.inputCfg)
			if tc.expectedErrMsg != "" {
				must.EqError(t, err, tc.expectedErrMsg)
				must.Nil(t, id)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expectedKey, id.Key())
				must.Eq(t, tc.expectedValue, id.Value())
			}
		})
	}
}
