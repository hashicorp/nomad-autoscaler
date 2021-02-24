package scaleutils

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils/nodepool"
	"github.com/stretchr/testify/assert"
)

func Test_classClusterPoolIdentifier(t *testing.T) {
	testCases := []struct {
		inputCfg            map[string]string
		expectedOutputPI    nodepool.ClusterNodePoolIdentifier
		expectedOutputError error
		name                string
	}{
		{
			inputCfg:            map[string]string{},
			expectedOutputPI:    nil,
			expectedOutputError: errors.New(`required config param "node_class" not set`),
			name:                "node_class cfg param not set",
		},
		{
			inputCfg:            map[string]string{"node_class": "my_pet_server"},
			expectedOutputPI:    nodepool.NewNodeClassPoolIdentifier("my_pet_server"),
			expectedOutputError: nil,
			name:                "node_class cfg param set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPI, actualError := classClusterPoolIdentifier(tc.inputCfg)
			assert.Equal(t, tc.expectedOutputPI, actualPI, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
		})
	}
}

func Test_autoscalerNodeID(t *testing.T) {
	testCases := []struct {
		envVar               bool
		expectedOutputString string
		expectedOutputError  error
		name                 string
	}{
		{
			envVar:               false,
			expectedOutputString: "",
			expectedOutputError:  nil,
			name:                 "no alloc ID found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualString, actualError := autoscalerNodeID(nil)
			assert.Equal(t, tc.expectedOutputString, actualString, tc.name)
			assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
		})
	}
}
