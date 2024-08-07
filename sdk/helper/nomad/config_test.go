// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"testing"
	"time"

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
				"nomad_address":               "vlc.nomad",
				"nomad_region":                "espana",
				"nomad_namespace":             "picassent",
				"nomad_token":                 "my-precious",
				"nomad_http-auth":             "username:password",
				"nomad_ca-cert":               "/etc/nomad.d/ca.crt",
				"nomad_ca-path":               "/etc/nomad.d/",
				"nomad_client-cert":           "/etc/nomad.d/client.crt",
				"nomad_client-key":            "/etc/nomad.d/client.key",
				"nomad_tls-server-name":       "lord-of-the-servers",
				"nomad_skip-verify":           "true",
				"nomad_block-query-wait-time": "60000ms",
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
				WaitTime: 1 * time.Minute,
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
		inputConfig       *config.Nomad
		expectedOutputMap map[string]string
		name              string
	}{
		{
			inputMap: map[string]string{},
			inputConfig: &config.Nomad{
				Address:            "test",
				Region:             "test",
				Namespace:          "test",
				Token:              "test",
				HTTPAuth:           "test:test",
				CACert:             "test",
				CAPath:             "test",
				ClientCert:         "test",
				ClientKey:          "test",
				TLSServerName:      "test",
				SkipVerify:         true,
				BlockQueryWaitTime: 2 * time.Minute,
			},
			expectedOutputMap: map[string]string{
				"nomad_address":               "test",
				"nomad_region":                "test",
				"nomad_namespace":             "test",
				"nomad_token":                 "test",
				"nomad_http-auth":             "test:test",
				"nomad_ca-cert":               "test",
				"nomad_ca-path":               "test",
				"nomad_client-cert":           "test",
				"nomad_client-key":            "test",
				"nomad_tls-server-name":       "test",
				"nomad_skip-verify":           "true",
				"nomad_block-query-wait-time": "2m0s",
			},
			name: "empty input map",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			MergeMapWithAgentConfig(tc.inputMap, tc.inputConfig)
			assert.Equal(t, tc.expectedOutputMap, tc.inputMap, tc.name)
		})
	}
}

func Test_MergeDefaultWithAgentConfig(t *testing.T) {
	testCases := []struct {
		inputConfig    *config.Nomad
		expectedOutput *api.Config
		name           string
	}{
		{
			inputConfig:    &config.Nomad{},
			expectedOutput: api.DefaultConfig(),
			name:           "default Autoscaler Nomad config",
		},
		{
			inputConfig: &config.Nomad{
				Address:            "http://demo.nomad:4646",
				Region:             "vlc",
				Namespace:          "platform",
				Token:              "shhhhhhhh",
				HTTPAuth:           "admin:admin",
				CACert:             "/path/to/long-lived/ca-cert",
				CAPath:             "/path/to/long-lived/",
				ClientCert:         "/path/to/long-lived/client-cert",
				ClientKey:          "/path/to/long-lived/key-cert",
				TLSServerName:      "whatdoesthisdo",
				SkipVerify:         true,
				BlockQueryWaitTime: 5 * time.Second,
			},
			expectedOutput: &api.Config{
				Address:   "http://demo.nomad:4646",
				Region:    "vlc",
				SecretID:  "shhhhhhhh",
				Namespace: "platform",
				HttpAuth: &api.HttpBasicAuth{
					Username: "admin",
					Password: "admin",
				},
				TLSConfig: &api.TLSConfig{
					CACert:        "/path/to/long-lived/ca-cert",
					CAPath:        "/path/to/long-lived/",
					ClientCert:    "/path/to/long-lived/client-cert",
					ClientKey:     "/path/to/long-lived/key-cert",
					TLSServerName: "whatdoesthisdo",
					Insecure:      true,
				},
				WaitTime: 5 * time.Second,
			},
			name: "full Autoscaler Nomad config override",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := MergeDefaultWithAgentConfig(tc.inputConfig)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
