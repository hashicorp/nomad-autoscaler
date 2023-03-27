// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"errors"
	"testing"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func TestNewClusterScaleUtils_drainSpec(t *testing.T) {
	testCases := []struct {
		inputCfg            map[string]string
		expectedOutputSpec  *api.DrainSpec
		expectedOutputError *multierror.Error
		name                string
	}{
		{
			inputCfg: map[string]string{},
			expectedOutputSpec: &api.DrainSpec{
				Deadline:         15 * time.Minute,
				IgnoreSystemJobs: false,
			},
			expectedOutputError: nil,
			name:                "no user parameters set",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline":           "10m",
				"node_drain_ignore_system_jobs": "true",
			},
			expectedOutputSpec: &api.DrainSpec{
				Deadline:         10 * time.Minute,
				IgnoreSystemJobs: true,
			},
			expectedOutputError: nil,
			name:                "all parameters set in config",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline": "10mm",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New(`time: unknown unit "mm" in duration "10mm"`)},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "config deadline parse error",
		},
		{
			inputCfg: map[string]string{
				"node_drain_ignore_system_jobs": "maybe",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New(`strconv.ParseBool: parsing "maybe": invalid syntax`)},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "config ignore system jobs parse error",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline":           "10mm",
				"node_drain_ignore_system_jobs": "maybe",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors: []error{
					errors.New(`time: unknown unit "mm" in duration "10mm"`),
					errors.New(`strconv.ParseBool: parsing "maybe": invalid syntax`),
				},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "multi config params parse error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDrainSpec, actualError := drainSpec(tc.inputCfg)
			assert.Equal(t, tc.expectedOutputSpec, actualDrainSpec, tc.name)
			if tc.expectedOutputError != nil {
				assert.EqualError(t, tc.expectedOutputError, actualError.Error(), tc.name)
			}
		})
	}
}
