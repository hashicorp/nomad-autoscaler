package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_codedErrorImpl(t *testing.T) {
	testCases := []struct {
		inputCode      int
		inputMsg       string
		expectedOutput *codedErrorImpl
		name           string
	}{
		{
			inputCode: 404,
			inputMsg:  "that doesn't seem to be here",
			expectedOutput: &codedErrorImpl{
				s:    "that doesn't seem to be here",
				code: 404,
			},
			name: "404 not found with message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			codedError := newCodedError(tc.inputCode, tc.inputMsg)
			assert.Equal(t, tc.inputCode, codedError.code, tc.name)
			assert.Equal(t, tc.inputMsg, codedError.s, tc.name)
		})
	}
}
