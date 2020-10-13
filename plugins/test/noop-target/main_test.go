package main

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	target := &Noop{logger: hclog.Default()}

	cases := []struct {
		name     string
		config   map[string]string
		expected *sdk.TargetStatus
		err      bool
	}{
		{
			name: "ready",
			config: map[string]string{
				"ready": "true",
			},
			expected: &sdk.TargetStatus{
				Ready: true,
			},
		},
		{
			name: "not ready",
			config: map[string]string{
				"ready": "false",
			},
			expected: &sdk.TargetStatus{
				Ready: false,
			},
		},
		{
			name: "with count",
			config: map[string]string{
				"count": "10",
			},
			expected: &sdk.TargetStatus{
				Ready: true,
				Count: 10,
			},
		},
		{
			name: "invalid count",
			config: map[string]string{
				"count": "not-a-number",
			},
			expected: &sdk.TargetStatus{
				Ready: true,
				Count: 0,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert := assert.New(t)

			got, err := target.Status(c.config)
			assert.Equal(got, c.expected)
			if c.err {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
