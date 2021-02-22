package scaleutils

import (
	"errors"
	"testing"

	multierror "github.com/hashicorp/go-multierror"
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
			assert.Equal(t, tc.expectedOutput, multiErrorFunc(tc.inputErr), tc.name)
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
			output := formattedMultiError(tc.inputErr)
			if tc.inputErr != nil {
				assert.NotNil(t, output, tc.name)
			} else {
				assert.Nil(t, output, tc.name)
			}
		})
	}
}
