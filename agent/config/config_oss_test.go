package config

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestValidateEntSubtree(t *testing.T) {
	// Inject a slice so we can test it.
	// TODO: remove this once we have a real slice config.
	entOnlyConfig = append(entOnlyConfig, "apm -> config")

	testCases := []struct {
		name            string
		path            string
		subtree         interface{}
		expectedInvalid []string
	}{
		{
			name:            "nil",
			path:            "",
			subtree:         nil,
			expectedInvalid: []string{},
		},
		{
			name:            "empty",
			path:            "",
			subtree:         &Agent{},
			expectedInvalid: []string{},
		},
		{
			name:            "not pointer",
			path:            "",
			subtree:         Agent{},
			expectedInvalid: []string{},
		},
		{
			name: "no enterprise",
			path: "",
			subtree: Agent{
				LogLevel: "INFO",
				Policy: &Policy{
					Dir: "./policies_dir",
				},
			},
			expectedInvalid: []string{},
		},
		{
			name: "subtree without enterprise",
			path: "policy",
			subtree: &Policy{
				Dir: "./policies_dir",
			},
			expectedInvalid: []string{},
		},
		{
			name:            "empty enterprise subtree",
			path:            "dynamic_application_sizing",
			subtree:         &DynamicApplicationSizing{},
			expectedInvalid: []string{},
		},
		{
			name: "with enterprise",
			path: "",
			subtree: &Agent{
				DynamicApplicationSizing: &DynamicApplicationSizing{
					MemoryMetric: "my_label",
				},
			},
			expectedInvalid: []string{
				"dynamic_application_sizing -> memory_metric",
			},
		},
		{
			name: "subtree with enterprise",
			path: "dynamic_application_sizing",
			subtree: &DynamicApplicationSizing{
				MemoryMetric: "my_label",
			},
			expectedInvalid: []string{
				"dynamic_application_sizing -> memory_metric",
			},
		},
		{
			name: "slice",
			path: "",
			subtree: &Agent{
				APMs: []*Plugin{
					{
						Config: map[string]string{
							"a": "b",
						},
					},
				},
			},
			expectedInvalid: []string{
				"apm -> config",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEntSubtree(tc.path, tc.subtree)

			if len(tc.expectedInvalid) == 0 {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)

			if mErr, ok := err.(*multierror.Error); ok {
				// Check if the expected errors are present in error list.
			OUTER:
				for _, invalid := range tc.expectedInvalid {
					for _, e := range mErr.Errors {
						if strings.Contains(e.Error(), invalid) {
							continue OUTER
						}
					}
					t.Fatalf("expected error for %q", invalid)
				}

				// Check that _only_ expected errors are present in error list.
			OUTER_2:
				for _, e := range mErr.Errors {
					for _, invalid := range tc.expectedInvalid {
						if strings.Contains(e.Error(), invalid) {
							continue OUTER_2
						}
					}
					t.Fatalf("unexpected error: %v", e)
				}
			}
		})
	}
}
