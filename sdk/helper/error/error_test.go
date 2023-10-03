// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package error

import (
	"errors"
	"testing"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/shoenig/test"
	"github.com/stretchr/testify/assert"
)

func Test_multiErrorFunc(t *testing.T) {
	testCases := []struct {
		inputErr       []error
		expectedOutput string
		name           string
	}{
		{
			inputErr: []error{
				errors.New("hello"),
				errors.New("is it me you're looking for"),
				errors.New("cause I wonder where you are"),
			},
			expectedOutput: "hello, is it me you're looking for, cause I wonder where you are",
			name:           "multiple input errors",
		},
		{
			inputErr:       []error{errors.New("this is not an exciting test message")},
			expectedOutput: "this is not an exciting test message",
			name:           "single input error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedOutput, MultiErrorFunc(tc.inputErr), tc.name)
		})
	}
}

func Test_formattedMultiError(t *testing.T) {
	testCases := []struct {
		inputErr *multierror.Error
		name     string
	}{
		{
			inputErr: nil,
			name:     "nil input error",
		},
		{
			inputErr: &multierror.Error{
				Errors: []error{errors.New("some ambiguous error")},
			},
			name: "non-nil input error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := FormattedMultiError(tc.inputErr)
			if tc.inputErr != nil {
				assert.NotNil(t, output, tc.name)
			} else {
				assert.Nil(t, output, tc.name)
			}
		})
	}
}

type statusCoder struct {
	err  string
	code int
}

func (sc statusCoder) Error() string {
	return sc.err
}

func (sc statusCoder) StatusCode() int {
	return sc.code
}

func (sc statusCoder) Unwrap() error {
	return sc
}

func Test_APIErrIs(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   int
		str    string
		expect bool
	}{
		{
			name:   "neither code nor str",
			err:    errors.New("nopers"),
			code:   100,
			str:    "aw jeez",
			expect: false,
		},
		{
			name: "code true",
			err: statusCoder{
				err:  "anything",
				code: 100,
			},
			code:   100,
			str:    "something else",
			expect: true,
		},
		{
			name: "code false",
			err: statusCoder{
				err:  "one thing",
				code: 100,
			},
			code:   200,
			str:    "something else",
			expect: false,
		},
		{
			name:   "str true",
			err:    errors.New("a bee c"),
			code:   0,
			str:    "bee",
			expect: true,
		},
		{
			name:   "str false",
			err:    errors.New("a bee c"),
			code:   0,
			str:    "zee",
			expect: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := APIErrIs(tc.err, tc.code, tc.str)
			if tc.expect {
				test.True(t, got)
			} else {
				test.False(t, got)
			}
		})
	}
}
