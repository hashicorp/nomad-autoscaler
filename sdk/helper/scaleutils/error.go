package scaleutils

import (
	"strings"

	multierror "github.com/hashicorp/go-multierror"
)

// multiErrorFunc is a helper to convert the standard multierror output into
// something a little more friendly to consoles. This is currently only used by
// the node filter, but could be more useful elsewhere in the future.
func multiErrorFunc(err []error) string {
	points := make([]string, len(err))
	for i, err := range err {
		points[i] = err.Error()
	}
	return strings.Join(points, ", ")
}

// formattedMultiError wraps any non-nil multierrors with the multiErrorFunc.
// It is safe to call in cases where the err may or may not be nil and will
// overwrite the existing formatter.
func formattedMultiError(err *multierror.Error) error {
	if err != nil {
		err.ErrorFormat = multiErrorFunc
	}
	return err.ErrorOrNil()
}
