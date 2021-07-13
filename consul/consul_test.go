package consul

import (
	"net"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

func Test_generateCheckID(t *testing.T) {
	testCases := []struct {
		inputReg       api.AgentCheckRegistration
		inputID        string
		expectedOutput string
		name           string
	}{
		{
			inputReg: api.AgentCheckRegistration{
				Name:      "nomad-autoscaler-agent",
				ServiceID: "_nomad-autoscaler_04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
				AgentServiceCheck: api.AgentServiceCheck{
					TCP:      net.JoinHostPort("http://127.0.0.1", "8080"),
					Interval: "10s",
				},
			},
			inputID:        "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			expectedOutput: "lak373nkrij2mipi7c2dxxcdcjka44jb",
			name:           "generic 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := generateCheckID(tc.inputReg, tc.inputID)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
