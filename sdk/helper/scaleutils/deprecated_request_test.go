// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"errors"
	"testing"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestScaleInReq_validate(t *testing.T) {
	testCases := []struct {
		inputScaleInReq     *ScaleInReq
		expectedOutputError error
		name                string
	}{
		{
			inputScaleInReq: &ScaleInReq{
				Num:           2,
				DrainDeadline: 20 * time.Minute,
				PoolIdentifier: &PoolIdentifier{
					IdentifierKey: "class",
					Value:         "myclass",
				},
				RemoteProvider: "aws",
				NodeIDStrategy: "newest_create_index",
			},
			expectedOutputError: nil,
			name:                "valid request",
		},
		{
			inputScaleInReq: &ScaleInReq{
				DrainDeadline: 20 * time.Minute,
				PoolIdentifier: &PoolIdentifier{
					IdentifierKey: "class",
					Value:         "myclass",
				},
				RemoteProvider: "aws",
				NodeIDStrategy: "newest_create_index",
			},
			expectedOutputError: &multierror.Error{
				Errors: []error{errors.New("num should be positive and non-zero")},
			},
			name: "missing num param",
		},
		{
			inputScaleInReq: &ScaleInReq{
				Num: 2,
				PoolIdentifier: &PoolIdentifier{
					IdentifierKey: "class",
					Value:         "myclass",
				},
				RemoteProvider: "aws",
				NodeIDStrategy: "newest_create_index",
			},
			expectedOutputError: &multierror.Error{
				Errors: []error{errors.New("deadline should be non-zero")},
			},
			name: "missing drain deadline",
		},
		{
			inputScaleInReq: &ScaleInReq{
				Num:            2,
				DrainDeadline:  20 * time.Minute,
				RemoteProvider: "aws",
				NodeIDStrategy: "newest_create_index",
			},
			expectedOutputError: &multierror.Error{
				Errors: []error{errors.New("pool identifier should be non-nil")},
			},
			name: "missing pool identifier",
		},
		{
			inputScaleInReq: &ScaleInReq{
				Num:           2,
				DrainDeadline: 20 * time.Minute,
				PoolIdentifier: &PoolIdentifier{
					IdentifierKey: "class",
					Value:         "myclass",
				},
				NodeIDStrategy: "newest_create_index",
			},
			expectedOutputError: &multierror.Error{
				Errors: []error{errors.New("remote provider should be set")},
			},
			name: "missing remote provider",
		},
		{
			inputScaleInReq: &ScaleInReq{
				Num:           2,
				DrainDeadline: 20 * time.Minute,
				PoolIdentifier: &PoolIdentifier{
					IdentifierKey: "class",
					Value:         "myclass",
				},
				RemoteProvider: "aws",
			},
			expectedOutputError: &multierror.Error{
				Errors: []error{errors.New("node ID strategy should be set")},
			},
			name: "missing node ID strategy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputScaleInReq.validate()
			assert.Equal(t, tc.expectedOutputError, actualOutput, tc.name)
		})
	}
}
