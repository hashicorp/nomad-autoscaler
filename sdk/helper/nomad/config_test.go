package nomad

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_ConfigFromNamespacedMap(t *testing.T) {
	testCases := []struct {
		inputCfg       map[string]string
		expectedOutput *api.Config
	}{
		{
			inputCfg: map[string]string{
				"nomad_address":         "vlc.nomad",
				"nomad_region":          "espana",
				"nomad_namespace":       "picassent",
				"nomad_token":           "my-precious",
				"nomad_http-auth":       "username:password",
				"nomad_ca-cert":         "/etc/nomad.d/ca.crt",
				"nomad_ca-path":         "/etc/nomad.d/",
				"nomad_client-cert":     "/etc/nomad.d/client.crt",
				"nomad_client-key":      "/etc/nomad.d/client.key",
				"nomad_tls-server-name": "lord-of-the-servers",
				"nomad_skip-verify":     "true",
			},
			expectedOutput: &api.Config{
				Address:   "vlc.nomad",
				Region:    "espana",
				SecretID:  "my-precious",
				Namespace: "picassent",
				HttpAuth: &api.HttpBasicAuth{
					Username: "username",
					Password: "password",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "/etc/nomad.d/ca.crt",
					CAPath:        "/etc/nomad.d/",
					ClientCert:    "/etc/nomad.d/client.crt",
					ClientKey:     "/etc/nomad.d/client.key",
					TLSServerName: "lord-of-the-servers",
					Insecure:      true,
				},
			},
		},
	}

	for _, tc := range testCases {
		actualOutput := ConfigFromNamespacedMap(tc.inputCfg)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

func Test_HTTPAuthFromString(t *testing.T) {
	testCases := []struct {
		inputAuth      string
		expectedOutput *api.HttpBasicAuth
	}{
		{
			inputAuth:      "",
			expectedOutput: nil,
		},
		{
			inputAuth:      "just-a-username",
			expectedOutput: &api.HttpBasicAuth{Username: "just-a-username"},
		},
		{
			inputAuth:      "username:password",
			expectedOutput: &api.HttpBasicAuth{Username: "username", Password: "password"},
		},
	}

	for _, tc := range testCases {
		actualOutput := HTTPAuthFromString(tc.inputAuth)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

func Test_MergeMapWithAgentConfig(t *testing.T) {
	testCases := []struct {
		inputMap          map[string]string
		inputAgentConfig  *config.Nomad
		expectedOutputMap map[string]string
		name              string
	}{
		{
			inputMap: map[string]string{},
			inputAgentConfig: &config.Nomad{
				Address:       "test",
				Region:        "test",
				Namespace:     "test",
				Token:         "test",
				HTTPAuth:      "test",
				CACert:        "test",
				CAPath:        "test",
				ClientCert:    "test",
				ClientKey:     "test",
				TLSServerName: "test",
				SkipVerify:    true,
			},
			expectedOutputMap: map[string]string{
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test",
				"nomad_ca-cert":         "test",
				"nomad_ca-path":         "test",
				"nomad_client-cert":     "test",
				"nomad_client-key":      "test",
				"nomad_tls-server-name": "test",
				"nomad_skip-verify":     "true",
			},
			name: "empty input map",
		},

		{
			inputMap: map[string]string{
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test",
				"nomad_ca-cert":         "test",
				"nomad_ca-path":         "test",
				"nomad_client-cert":     "test",
				"nomad_client-key":      "test",
				"nomad_tls-server-name": "test",
				"nomad_skip-verify":     "true",
			},
			inputAgentConfig: &config.Nomad{
				Address:       "test-new",
				Region:        "test-new",
				Namespace:     "test-new",
				Token:         "test-new",
				HTTPAuth:      "test-new",
				CACert:        "test-new",
				CAPath:        "test-new",
				ClientCert:    "test-new",
				ClientKey:     "test-new",
				TLSServerName: "test-new",
				SkipVerify:    false,
			},
			expectedOutputMap: map[string]string{
				"nomad_address":         "test",
				"nomad_region":          "test",
				"nomad_namespace":       "test",
				"nomad_token":           "test",
				"nomad_http-auth":       "test",
				"nomad_ca-cert":         "test",
				"nomad_ca-path":         "test",
				"nomad_client-cert":     "test",
				"nomad_client-key":      "test",
				"nomad_tls-server-name": "test",
				"nomad_skip-verify":     "true",
			},
			name: "fully populated input map and input agent config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			MergeMapWithAgentConfig(tc.inputMap, tc.inputAgentConfig)
			assert.Equal(t, tc.expectedOutputMap, tc.inputMap, tc.name)
		})
	}
}
