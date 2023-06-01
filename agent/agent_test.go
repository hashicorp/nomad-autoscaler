// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestAgent_generateNomadClient(t *testing.T) {
	testCases := []struct {
		inputAgent       *Agent
		expectedOutputEr error
		name             string
	}{
		{
			inputAgent: &Agent{
				nomadCfg: api.DefaultConfig(),
			},
			expectedOutputEr: nil,
			name:             "default Nomad API config input",
		},
		{
			inputAgent: &Agent{
				nomadCfg: &api.Config{
					Address: "\t",
				},
			},
			expectedOutputEr: errors.New(`failed to instantiate Nomad client: invalid address '	': parse "\t": net/url: invalid control character in URL`),
			name:             "invalid input Nomad address", //nolint
		},
	}

	for _, tc := range testCases {
		actualOutputErr := tc.inputAgent.generateNomadClient()
		assert.Equal(t, tc.expectedOutputEr, actualOutputErr, tc.name)
		if actualOutputErr == nil {
			assert.Equal(t, tc.inputAgent.nomadCfg.Address, tc.inputAgent.nomadClient.Address(), tc.name)
		}
	}
}
