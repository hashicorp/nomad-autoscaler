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
				must.EqError(t, err, tc.expectedOutputErr.Error())
				must.Nil(t, ids)
			} else {
				must.NoError(t, err)
				must.Len(t, tc.expectedOutputLen, ids)
			}
		})
	}
}

func Test_NewClusterNodePoolIdentifier(t *testing.T) {
	testCases := []struct {
		inputCfg            map[string]string
		expectedOutputKey   string
		expectedOutputValue string
		expectedOutputErr   error
		name                string
	}{
		{
			inputCfg:            map[string]string{},
			expectedOutputKey:   "",
			expectedOutputValue: "",
			expectedOutputErr:   errors.New("node pool identification method required"),
			name:                "empty input config",
		},
		{
			inputCfg:            map[string]string{"datacentre": "dc1"},
			expectedOutputKey:   "",
			expectedOutputValue: "",
			expectedOutputErr:   errors.New("node pool identification method required"),
			name:                "input config with incorrect key",
		},
		{
			inputCfg:            map[string]string{"datacenter": "dc1"},
			expectedOutputKey:   "datacenter",
			expectedOutputValue: "dc1",
			expectedOutputErr:   nil,
			name:                "datacenter configured in config",
		},
		{
			inputCfg:            map[string]string{"node_class": "high-memory"},
			expectedOutputKey:   "node_class",
			expectedOutputValue: "high-memory",
			expectedOutputErr:   nil,
			name:                "node_class configured in config",
		},
		{
			inputCfg:            map[string]string{"node_pool": "gpu"},
			expectedOutputKey:   "node_pool",
			expectedOutputValue: "gpu",
			expectedOutputErr:   nil,
			name:                "node_pool configured in config",
		},
		{
			inputCfg:            map[string]string{"node_class": "high-memory", "datacenter": "dc1"},
			expectedOutputKey:   "combined_identifier",
			expectedOutputValue: "node_class:high-memory and datacenter:dc1",
			expectedOutputErr:   nil,
			name:                "node_class and datacenter are configured in config",
		},
		{
			inputCfg:            map[string]string{"node_pool": "gpu", "datacenter": "dc1"},
			expectedOutputKey:   "combined_identifier",
			expectedOutputValue: "datacenter:dc1 and node_pool:gpu",
			expectedOutputErr:   nil,
			name:                "node_pool and datacenter are configured in config",
		},
		{
			inputCfg: map[string]string{
				"node_class": "high-memory",
				"node_pool":  "gpu",
				"datacenter": "dc1",
			},
			expectedOutputKey:   "combined_identifier",
			expectedOutputValue: "node_class:high-memory and datacenter:dc1 and node_pool:gpu",
			expectedOutputErr:   nil,
			name:                "node_class, node_pool, and datacenter are configured in config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl, err := NewClusterNodePoolIdentifier(tc.inputCfg)
			if tc.expectedOutputErr != nil {
				assert.Equal(t, tc.expectedOutputErr, err, tc.name)
			} else {
				assert.NotNil(t, impl, tc.name)
				assert.Equal(t, tc.expectedOutputKey, impl.Key(), tc.name)
				assert.Equal(t, tc.expectedOutputValue, impl.Value(), tc.name)
			}
		})
	}
}
