package nomad

import (
	"testing"

	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_ConfigToMap(t *testing.T) {
	testCases := []struct {
		inputCfg       *config.Nomad
		expectedOutput map[string]string
	}{
		{
			inputCfg: &config.Nomad{
				Address:       "vlc.nomad",
				Region:        "espana",
				Namespace:     "picassent",
				Token:         "my-precious",
				HTTPAuth:      "username:password",
				CACert:        "/etc/nomad.d/ca.crt",
				CAPath:        "/etc/nomad.d/",
				ClientCert:    "/etc/nomad.d/client.crt",
				ClientKey:     "/etc/nomad.d/client.key",
				TLSServerName: "lord-of-the-servers",
				SkipVerify:    true,
			},
			expectedOutput: map[string]string{
				"address":         "vlc.nomad",
				"region":          "espana",
				"namespace":       "picassent",
				"token":           "my-precious",
				"http-auth":       "username:password",
				"ca-cert":         "/etc/nomad.d/ca.crt",
				"ca-path":         "/etc/nomad.d/",
				"client-cert":     "/etc/nomad.d/client.crt",
				"client-key":      "/etc/nomad.d/client.key",
				"tls-server-name": "lord-of-the-servers",
				"skip-verify":     "true",
			},
		},
	}

	for _, tc := range testCases {
		actualOutput := ConfigToMap(tc.inputCfg)
		assert.Equal(t, tc.expectedOutput, actualOutput)
	}
}

func Test_ConfigFromMap(t *testing.T) {
	testCases := []struct {
		inputCfg       map[string]string
		expectedOutput *api.Config
	}{
		{
			inputCfg: map[string]string{
				"address":         "vlc.nomad",
				"region":          "espana",
				"namespace":       "picassent",
				"token":           "my-precious",
				"http-auth":       "username:password",
				"ca-cert":         "/etc/nomad.d/ca.crt",
				"ca-path":         "/etc/nomad.d/",
				"client-cert":     "/etc/nomad.d/client.crt",
				"client-key":      "/etc/nomad.d/client.key",
				"tls-server-name": "lord-of-the-servers",
				"skip-verify":     "true",
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
		actualOutput := ConfigFromMap(tc.inputCfg)
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
