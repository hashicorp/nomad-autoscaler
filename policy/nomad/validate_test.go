// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

var showValidationError = flag.Bool("show-validation-error", false, "")

func Test_validateScalingPolicy(t *testing.T) {
	testCases := []struct {
		name        string
		input       *api.ScalingPolicy
		inputFile   string
		expectError bool
	}{
		{
			name:        "valid policy",
			inputFile:   "full-scaling",
			expectError: false,
		},
		{
			name:        "valid min policy",
			inputFile:   "minimum-valid-scaling",
			expectError: false,
		},
		{
			name:        "nil policy",
			inputFile:   "missing-scaling",
			expectError: true,
		},
		{
			name:        "empty policy",
			input:       &api.ScalingPolicy{},
			expectError: true,
		},
		{
			name: "id is missing",
			input: &api.ScalingPolicy{
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "target is missing",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Min:  ptr.Of(int64(1)),
				Max:  ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "min is missing",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "min is negative",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(-1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "max is negative",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(-5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "max less than min",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(2)),
				Max: ptr.Of(int64(1)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name:        "policy is missing",
			inputFile:   "missing-policy",
			expectError: true,
		},
		{
			name:        "policy.check.source is not a string",
			inputFile:   "invalid-source-not-string",
			expectError: true,
		},
		{
			name:        "policy.check.query is missing",
			inputFile:   "invalid-missing-query",
			expectError: true,
		},
		{
			name:        "policy.check.query is not a string",
			inputFile:   "invalid-query",
			expectError: true,
		},
		{
			name:        "policy.check.query_window is not a string",
			inputFile:   "invalid-query-window1",
			expectError: true,
		},
		{
			name:        "policy.check.query_window is not a duration",
			inputFile:   "invalid-query-window2",
			expectError: true,
		},
		{
			name:        "policy.check.query is empty",
			inputFile:   "invalid-empty-query",
			expectError: true,
		},
		{
			name:        "policy.check.strategy is missing",
			inputFile:   "missing-strategy",
			expectError: true,
		},
		{
			name:        "policy.check.strategy without metrics",
			inputFile:   "strategy-without-metric",
			expectError: false,
		},
		{
			name: "policy.check.strategy.name is empty",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.check.strategy has wrong type",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": "not a block",
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name:        "policy.check.strategy multiple",
			inputFile:   "invalid-multiple-strategies",
			expectError: true,
		},
		{
			name:        "policy.evaluation_interval has wrong type",
			inputFile:   "invalid-evaluation-interval-type",
			expectError: true,
		},
		{
			name:        "policy.evaluation_interval has wrong format",
			inputFile:   "invalid-evaluation-interval",
			expectError: true,
		},
		{
			name: "policy.target.name is empty",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyTarget: []interface{}{
						map[string]interface{}{
							"": []interface{}{
								map[string]interface{}{
									"key": "value",
								},
							},
						},
					},
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "policy.target has wrong type",
			input: &api.ScalingPolicy{
				ID:   "id",
				Type: "horizontal",
				Target: map[string]string{
					"key": "value",
				},
				Min: ptr.Of(int64(1)),
				Max: ptr.Of(int64(5)),
				Policy: map[string]interface{}{
					keyTarget: []interface{}{
						map[string]interface{}{
							"target": "not block",
						},
					},
					keyChecks: []interface{}{
						map[string]interface{}{
							"check": []interface{}{
								map[string]interface{}{
									keySource: "source",
									keyQuery:  "query",
									keyStrategy: []interface{}{
										map[string]interface{}{
											"strategy": []interface{}{
												map[string]interface{}{
													"key": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name:        "policy.target multiple",
			inputFile:   "invalid-multiple-targets",
			expectError: true,
		},
		{
			name:        "policy.cooldown has wrong type",
			inputFile:   "invalid-cooldown-type",
			expectError: true,
		},
		{
			name:        "policy.cooldown has wrong format",
			inputFile:   "invalid-cooldown",
			expectError: true,
		},
		{
			name:        "policy.cooldown_on_scale_up has wrong type",
			inputFile:   "invalid-cooldown-on-scale-up-type",
			expectError: true,
		},
		{
			name:        "policy.cooldown_on_scale_up has wrong format",
			inputFile:   "invalid-cooldown-on-scale-up",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var input *api.ScalingPolicy

			// Read from file if defined, otherwise use input object.
			if tc.inputFile != "" {
				jobPath := fmt.Sprintf("test-fixtures/%s.json.golden", tc.inputFile)
				job := TestParseJob(t, jobPath)

				if len(job.TaskGroups) != 1 {
					t.Fatalf("expected 1 group, found %d", len(job.TaskGroups))
				}

				input = job.TaskGroups[0].Scaling
			} else {
				input = tc.input
			}

			err := validateScalingPolicy(input)

			// Print error if -show-validation-error flag is set.
			if err != nil && *showValidationError {
				fmt.Println(err)
			}

			if tc.expectError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func Test_validateBlock(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		validator   func(in map[string]interface{}, path string) error
		expectError bool
	}{
		{
			name: "valid block",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name: "valid block map",
			input: map[string]interface{}{
				"key": "value",
			},
			expectError: false,
		},
		{
			name:        "block root has wrong type",
			input:       true,
			expectError: true,
		},
		{
			name:        "block root is empty",
			input:       []interface{}{},
			expectError: true,
		},
		{
			name: "block root has more than 1 element",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
				1,
			},
			expectError: true,
		},
		{
			name:        "block root first element has wront type",
			input:       []interface{}{1},
			expectError: true,
		},
		{
			name: "validator is called",
			input: []interface{}{
				map[string]interface{}{
					"key": "value",
				},
			},
			validator: func(in map[string]interface{}, path string) error {
				return errors.New("error from validator")
			},
			expectError: true,
		},
		{
			name: "validator is called with map",
			input: map[string]interface{}{
				"key": "value",
			},
			validator: func(in map[string]interface{}, path string) error {
				return errors.New("error from validator")
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBlock(tc.input, "path.key", tc.validator)
			if err != nil && *showValidationError {
				fmt.Println(err)
			}

			assertFunc := must.NoError
			if tc.expectError {
				assertFunc = must.Error
			}

			assertFunc(t, err)
		})
	}
}
