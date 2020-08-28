package plugin

import (
	"errors"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestAPMPlugin_SetConfig(t *testing.T) {
	testCases := []struct {
		inputConfig  map[string]string
		expectOutput error
		name         string
	}{
		{
			inputConfig:  map[string]string{},
			expectOutput: errors.New(`"address" config value cannot be empty`),
			name:         "no required config parameters set",
		},
		{
			inputConfig:  map[string]string{"address": "\n\n"},
			expectOutput: errors.New(`failed to initialize Prometheus client: parse "\n\n": net/url: invalid control character in URL`),
			name:         "required config parameters set but value malformed",
		},
		{
			inputConfig:  map[string]string{"address": "http://127.0.0.1:9090"},
			expectOutput: nil,
			name:         "required and valid config parameters set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apmPlugin := APMPlugin{logger: hclog.NewNullLogger()}

			actualOutput := apmPlugin.SetConfig(tc.inputConfig)
			assert.Equal(t, tc.expectOutput, actualOutput, tc.name)

			// If the function call did not return an error, we should have a
			// non-nil Prometheus client.
			if actualOutput == nil {
				assert.NotNil(t, apmPlugin.client)
			}
		})
	}
}
