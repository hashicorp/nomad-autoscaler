package http

// errInvalidMethod is the error message used when a HTTP request is sent using
// the incorrect method.
const errInvalidMethod = "Invalid method"

// codedError defines the interface used for custom HTTP error handling. It
// ensures we include a message and response code to differentiate between
// internal and other errors.
type codedError interface {
	error
	Code() int
}

// Ensure codedErrorImpl satisfies the codedError interface.
var _ codedError = (*codedErrorImpl)(nil)

// codedErrorImpl implements the codedError interface.
type codedErrorImpl struct {
	s    string
	code int
}

// newCodedError creates a new coded error.
func newCodedError(c int, s string) *codedErrorImpl {
	return &codedErrorImpl{s, c}
}

func (e *codedErrorImpl) Error() string { return e.s }
func (e *codedErrorImpl) Code() int     { return e.code }
