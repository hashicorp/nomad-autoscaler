// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package error

import (
	"errors"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
)

// MultiErrorFunc is a helper to convert the standard multierror output into
// something a little more friendly to consoles.
func MultiErrorFunc(err []error) string {
	points := make([]string, len(err))
	for i, err := range err {
		points[i] = err.Error()
	}
	return strings.Join(points, ", ")
}

// FormattedMultiError wraps any non-nil multierrors with the multiErrorFunc.
// It is safe to call in cases where the err may or may not be nil and will
// overwrite the existing formatter.
func FormattedMultiError(err *multierror.Error) error {
	if err != nil {
		err.ErrorFormat = MultiErrorFunc
	}
	return err.ErrorOrNil()
}

// StatusCoder is an error with an extra StatusCode method.
// mainly, nomad api.UnexpectedResponseError implements this.
type StatusCoder interface {
	Error() string
	Unwrap() error
	StatusCode() int
}

// APIErrIs attempts to coerce err into an UnexpectedResponseError to check
// its status code. Failing that, it will look for str in the error string.
// If code==0 it will be ignored, same for str==""
func APIErrIs(err error, code int, str string) bool {
	if err == nil {
		return false
	}
	if code > 0 {
		var sc StatusCoder
		if errors.As(err, &sc) {
			return sc.StatusCode() == code
		}
	}
	return str != "" && strings.Contains(err.Error(), str)
}
