package policy

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/stretchr/testify/assert"
)

func TestAgent_getNomadAPMNames(t *testing.T) {
	testCases := []struct {
		inputAPMs      []*config.Plugin
		expectedOutput []string
		name           string
	}{
		{
			inputAPMs:      []*config.Plugin{},
			expectedOutput: nil,
			name:           "no Nomad APMs configured",
		},
		{
			inputAPMs: []*config.Plugin{
				{Name: "nomad-apm", Driver: "nomad-apm"},
			},
			expectedOutput: []string{"nomad-apm"},
			name:           "default Nomad APM configured",
		},
		{
			inputAPMs: []*config.Plugin{
				{Name: "nomad-platform-apm", Driver: "nomad-apm"},
				{Name: "nomad-qa-apm", Driver: "nomad-apm"},
				{Name: "nomad-operations-apm", Driver: "nomad-apm"},
			},
			expectedOutput: []string{"nomad-platform-apm", "nomad-qa-apm", "nomad-operations-apm"},
			name:           "default Nomad APM configured",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOuput := getNomadAPMNames(tc.inputAPMs)
			assert.Equal(t, tc.expectedOutput, actualOuput, tc.name)
		})
	}
}
