package agent

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAgent_generateConsulClient(t *testing.T) {
	require := require.New(t)
	agent := Agent{
		logger: hclog.NewNullLogger(),
		config: &config.Agent{
			HTTP: &config.HTTP{
				BindAddress: "127.0.0.1",
				BindPort:    1066,
			},
			Consul: nil,
		},
	}

	// nil config => no consul client
	require.NoError(agent.generateConsulClient())
	require.Nil(agent.consul, "client should be nil because config was nil")

	// empty config => default consul client
	agent.config.Consul = &config.Consul{}
	require.NoError(agent.generateConsulClient())
	require.NotNil(agent.consul)

	// explicit config overrides defaults, can throw errors
	agent.config.Consul.Addr = "bad://1234"
	require.EqualError(agent.generateConsulClient(), "failed to instantiate Consul client: Unknown protocol scheme: bad")
}
