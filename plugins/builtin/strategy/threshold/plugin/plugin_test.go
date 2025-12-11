// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThresholdPlugin(t *testing.T) {
	testCases := []struct {
		name           string
		count          int64
		metrics        []float64
		config         map[string]string
		expectedAction *sdk.ScalingAction
		expectedErr    string
	}{
		{
			name:    "lower_bound is inclusive",
			count:   1,
			metrics: []float64{10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "10",
				"delta":       "-1",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     0,
				Direction: sdk.ScaleDirectionDown,
				Reason:    "scaling down because metric is within bounds",
			},
		},
		{
			name:    "upper_bound is exclusive",
			count:   1,
			metrics: []float64{10, 10, 10, 10, 10},
			config: map[string]string{
				"upper_bound": "10",
				"delta":       "1",
			},
			expectedAction: &sdk.ScalingAction{
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "lower_bound is inclusive and upper_bound is exclusive/no action",
			count:   1,
			metrics: []float64{10, 10, 20, 20, 20},
			config: map[string]string{
				"lower_bound":           "10",
				"upper_bound":           "20",
				"delta":                 "1",
				"within_bounds_trigger": "3",
			},
			expectedAction: &sdk.ScalingAction{
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "lower_bound is inclusive and upper_bound is exclusive/scale",
			count:   1,
			metrics: []float64{10, 10, 20, 20, 20},
			config: map[string]string{
				"lower_bound":           "10",
				"upper_bound":           "20",
				"delta":                 "1",
				"within_bounds_trigger": "2",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     2,
				Direction: sdk.ScaleDirectionUp,
				Reason:    "scaling up because metric is within bounds",
			},
		},
		{
			name:    "delta scale up",
			count:   1,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"delta":       "1",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     2,
				Reason:    "scaling up because metric is within bounds",
				Direction: sdk.ScaleDirectionUp,
			},
		},
		{
			name:    "delta scale down",
			count:   1,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"delta":       "-1",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     0,
				Reason:    "scaling down because metric is within bounds",
				Direction: sdk.ScaleDirectionDown,
			},
		},
		{
			name:    "delta scale down, no negative",
			count:   0,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"delta":       "-1",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     0,
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "percentage scale up",
			count:   10,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"percentage":  "30",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     13,
				Reason:    "scaling up because metric is within bounds",
				Direction: sdk.ScaleDirectionUp,
			},
		},
		{
			name:    "percentage scale down",
			count:   10,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"percentage":  "-30",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     7,
				Reason:    "scaling down because metric is within bounds",
				Direction: sdk.ScaleDirectionDown,
			},
		},
		{
			name:    "percentage scale none",
			count:   10,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"percentage":  "0",
			},
			expectedAction: &sdk.ScalingAction{
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "value scale up",
			count:   1,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"value":       "10",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     10,
				Reason:    "scaling up because metric is within bounds",
				Direction: sdk.ScaleDirectionUp,
			},
		},
		{
			name:    "value scale down",
			count:   20,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"value":       "10",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     10,
				Reason:    "scaling down because metric is within bounds",
				Direction: sdk.ScaleDirectionDown,
			},
		},
		{
			name:    "percentage scale none",
			count:   10,
			metrics: []float64{10, 10, 10, 10, 10, 10},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"value":       "10",
			},
			expectedAction: &sdk.ScalingAction{
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "not enough metrics within bounds",
			count:   1,
			metrics: []float64{10, 0, 0, 0, 0, 0},
			config: map[string]string{
				"lower_bound": "5",
				"upper_bound": "20",
				"delta":       "1",
			},
			expectedAction: &sdk.ScalingAction{
				Direction: sdk.ScaleDirectionNone,
			},
		},
		{
			name:    "custom trigger value",
			count:   1,
			metrics: []float64{10, 0, 0, 0, 0, 0},
			config: map[string]string{
				"lower_bound":           "5",
				"upper_bound":           "20",
				"delta":                 "1",
				"within_bounds_trigger": "1",
			},
			expectedAction: &sdk.ScalingAction{
				Count:     2,
				Reason:    "scaling up because metric is within bounds",
				Direction: sdk.ScaleDirectionUp,
			},
		},
		{
			name:    "missing bounds",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"delta": "1",
			},
			expectedErr: `must have either "lower_bound" or "upper_bound"`,
		},
		{
			name:    "invalid upper bound",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"upper_bound": "not-a-number",
			},
			expectedErr: `invalid value for "upper_bound"`,
		},
		{
			name:    "invalid lower bound",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"lower_bound": "not-a-number",
			},
			expectedErr: `invalid value for "lower_bound"`,
		},
		{
			name:    "missing action",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"lower_bound": "5",
			},
			expectedErr: `must have either "delta", "percentage" or "value"`,
		},
		{
			name:    "too many actions",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"lower_bound": "5",
				"delta":       "1",
				"value":       "10",
			},
			expectedErr: `only one of "delta", "percentage" or "value" must be provided`,
		},
		{
			name:    "invalid trigger",
			count:   1,
			metrics: []float64{0},
			config: map[string]string{
				"lower_bound":           "5",
				"within_bounds_trigger": "not-a-number",
			},
			expectedErr: `invalid value for "within_bounds_trigger"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewThresholdPlugin(hclog.NewNullLogger())

			var metrics sdk.TimestampedMetrics
			for _, m := range tc.metrics {
				metrics = append(metrics, sdk.TimestampedMetric{Value: m})
			}

			eval := &sdk.ScalingCheckEvaluation{
				Action: &sdk.ScalingAction{},
				Check: &sdk.ScalingPolicyCheck{
					Name: tc.name,
					Strategy: &sdk.ScalingPolicyStrategy{
						Config: tc.config,
					},
				},
				Metrics: metrics,
			}

			got, err := p.Run(eval, tc.count)
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			if got == nil {
				assert.Nil(t, tc.expectedAction)
				return
			}

			assert.Equal(t, tc.expectedAction, got.Action)
		})
	}
}
