package error

import (
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
