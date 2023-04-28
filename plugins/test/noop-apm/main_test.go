// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	apm := &Noop{logger: hclog.Default()}
	now := time.Now()

	cases := []struct {
		name        string
		query       string
		timeRange   sdk.TimeRange
		expected    sdk.TimestampedMetrics
		expectedLen int
		err         bool
	}{
		{
			name:  "fixed query",
			query: "fixed:5",
			timeRange: sdk.TimeRange{
				From: now.Add(-10 * time.Second),
				To:   now,
			},
			expected: sdk.TimestampedMetrics{
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-9 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-8 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-7 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-6 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-5 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-4 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-3 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-2 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.Add(-1 * time.Second).UTC()},
				sdk.TimestampedMetric{Value: 5, Timestamp: now.UTC()},
			},
		},
		{
			name:  "random query",
			query: "random:1:30",
			timeRange: sdk.TimeRange{
				From: now.Add(-300 * time.Second),
				To:   now,
			},
			expectedLen: 300,
		},
		{
			name:  "invalid query type",
			query: "not-valid:1:30",
			err:   true,
		},
		{
			name:  "invalid query",
			query: "not-valid",
			err:   true,
		},
		{
			name:  "invalid fixed query",
			query: "fixed:2:30",
			err:   true,
		},
		{
			name:  "invalid random query",
			query: "random:2:30:1:1:2",
			err:   true,
		},
		{
			name:  "invalid random query",
			query: "random:2",
			err:   true,
		},
		{
			name:  "invalid random query numbers",
			query: "random:a:b",
			err:   true,
		},
		{
			name:  "invalid fixed query number",
			query: "fixed:a",
			err:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert := assert.New(t)

			got, err := apm.Query(c.query, c.timeRange)
			if c.err {
				assert.Error(err)
				assert.Nil(got)
				return
			}

			assert.NoError(err)

			parts := strings.Split(c.query, ":")
			switch parts[0] {
			case "fixed":
				assert.Equal(c.expected, got)
			case "random":
				start, err := strconv.ParseFloat(parts[1], 10)
				end, err := strconv.ParseFloat(parts[2], 10)
				assert.NoError(err)

				min := start
				max := end
				for _, m := range got {
					if m.Value > max {
						max = m.Value
					}
					if m.Value < min {
						min = m.Value
					}
				}

				assert.Len(got, c.expectedLen)
				assert.GreaterOrEqual(min, start)
				assert.LessOrEqual(max, end)
			}
		})
	}
}
