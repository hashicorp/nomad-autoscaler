// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package modes

import (
	"flag"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestChecker_flags(t *testing.T) {
	testCases := []struct {
		name         string
		modesEnabled []string
		expected     map[string][]string
	}{
		{
			name:         "none",
			modesEnabled: []string{},
			expected: map[string][]string{
				"-all":        {"Nomad Autoscaler Enterprise", "Nomad Autoscaler Pro", "Nomad Autoscaler Expert"},
				"-ent":        {"Nomad Autoscaler Enterprise"},
				"-ent-pro":    {"Nomad Autoscaler Enterprise", "Nomad Autoscaler Pro"},
				"-pro-expert": {"Nomad Autoscaler Pro", "Nomad Autoscaler Expert"},
			},
		},
		{
			name:         "ent",
			modesEnabled: []string{"ent"},
			expected: map[string][]string{
				"-pro-expert": {"Nomad Autoscaler Pro", "Nomad Autoscaler Expert"},
			},
		},
		{
			name:         "pro",
			modesEnabled: []string{"pro"},
			expected: map[string][]string{
				"-ent": {"Nomad Autoscaler Enterprise"},
			},
		},
		{
			name:         "ent pro",
			modesEnabled: []string{"ent", "pro"},
			expected:     map[string][]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modeChecker := NewChecker(TestModes, tc.modesEnabled)

			flags := flag.NewFlagSet("", flag.ContinueOnError)
			config := NewTestStruct()

			modeChecker.Flag("none", []string{}, func(name string) {
				flags.BoolVar(&config.TopLevelNone, name, false, "")
			})
			modeChecker.Flag("ent", []string{"ent"}, func(name string) {
				flags.BoolVar(&config.TopLevelNone, name, false, "")
			})
			modeChecker.Flag("ent-pro", []string{"ent", "pro"}, func(name string) {
				flags.BoolVar(&config.TopLevelNone, name, false, "")
			})
			modeChecker.Flag("pro-expert", []string{"pro", "expert"}, func(name string) {
				flags.BoolVar(&config.TopLevelNone, name, false, "")
			})
			modeChecker.Flag("all", []string{"ent", "pro", "expert"}, func(name string) {
				flags.BoolVar(&config.TopLevelNone, name, false, "")
			})

			args := []string{
				"-none",
				"-ent",
				"-ent-pro",
				"-pro-expert",
				"-all",
			}

			err := flags.Parse(args)
			assert.NoError(t, err)

			err = modeChecker.ValidateFlags(flags)
			if len(tc.expected) == 0 {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			assert.Len(t, err.(*multierror.Error).Errors, len(tc.expected))

			for _, e := range err.(*multierror.Error).Errors {
				flag := strings.Split(e.Error(), " ")[0]
				for _, name := range tc.expected[flag] {
					assert.Contains(t, e.Error(), name)
				}
			}
		})
	}
}
