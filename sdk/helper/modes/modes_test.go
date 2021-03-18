package modes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecker_isAllowed(t *testing.T) {
	testCases := []struct {
		name    string
		enabled []string
		input   []string
		allowed bool
	}{
		{
			name:    "nil",
			enabled: nil,
			input:   nil,
			allowed: true,
		},
		{
			name:    "none enable with empty input",
			enabled: []string{},
			input:   []string{},
			allowed: true,
		},
		{
			name:    "none enabled with some input",
			enabled: []string{},
			input:   []string{"ent"},
			allowed: false,
		},
		{
			name:    "ent enabled with empty input",
			enabled: []string{"ent"},
			input:   []string{},
			allowed: true,
		},
		{
			name:    "ent enabled with ent input",
			enabled: []string{"ent"},
			input:   []string{"ent"},
			allowed: true,
		},
		{
			name:    "ent enabled with ent input and other",
			enabled: []string{"ent"},
			input:   []string{"ent", "pro"},
			allowed: true,
		},
		{
			name:    "two enabled with empty input",
			enabled: []string{"ent", "pro"},
			input:   []string{},
			allowed: true,
		},
		{
			name:    "two enabled with one in input",
			enabled: []string{"ent", "pro"},
			input:   []string{"ent"},
			allowed: true,
		},
		{
			name:    "two enabled with input not allowed",
			enabled: []string{"ent", "pro"},
			input:   []string{"expert"},
			allowed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewChecker(TestModes, tc.enabled)
			got := c.isAllowed(tc.input)
			assert.Equal(t, tc.allowed, got)
		})
	}
}
