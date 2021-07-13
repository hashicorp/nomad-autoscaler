package consul

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_generateServiceID(t *testing.T) {
	testCases := []struct {
		inputConsul    *Consul
		expectedOutput string
		name           string
	}{
		{
			inputConsul: &Consul{
				agentID:     "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
				serviceName: "nomad-autoscaler",
			},
			expectedOutput: "_nomad-autoscaler_04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			name:           "default styled input",
		},
		{
			inputConsul: &Consul{
				agentID:     "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
				serviceName: "i_called-this!what{i.wanted",
			},
			expectedOutput: "_i_called-this!what{i.wanted_04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			name:           "non-default styled input",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputConsul.generateServiceID()
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func Test_agentIDFromServiceID(t *testing.T) {
	testCases := []struct {
		inputServiceID string
		expectedOutput string
		name           string
	}{
		{
			inputServiceID: "_nomad-autoscaler_04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			expectedOutput: "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			name:           "default styled input",
		},
		{
			inputServiceID: "_i_called-this!what{i.wanted_04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			expectedOutput: "04a303cf-9cd2-8faf-02e3-2ad3a387cd55",
			name:           "non-default styled input",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := agentIDFromServiceID(tc.inputServiceID)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
