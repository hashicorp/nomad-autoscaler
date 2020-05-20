package agent

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/stretchr/testify/assert"
)

func TestAgent_setupPluginConfig(t *testing.T) {
	c, err := config.Default()
	assert.Nil(t, err)

	a := NewAgent(c, hclog.NewNullLogger())

	expectedOutput := map[string][]*config.Plugin{
		"target": {
			{
				Name:   "nomad-target",
				Driver: "nomad-target",
				Config: map[string]string{"address": "http://127.0.0.1:4646", "region": "global"},
			},
		},
		"apm": {
			{
				Name:   "nomad-apm",
				Driver: "nomad-apm",
				Config: map[string]string{"address": "http://127.0.0.1:4646", "region": "global"},
			},
		},
		"strategy": {
			{
				Name:   "target-value",
				Driver: "target-value",
				Config: nil,
			},
		},
	}

	assert.Equal(t, expectedOutput, a.setupPluginConfig())
}
