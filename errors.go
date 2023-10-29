package structhttp

type (
	// HTTPStatusCoder is an interface for errors that can return an
	// HTTP status code.
	HTTPStatusCoder interface {
		HTTPStatusCode() int
	}

	// Error is an error that can return an HTTP status code.
	Error struct {
		StatusCode int
		Err        error
	}
)

// NewError returns a new Error with the given status code and wrapped
// error.
func NewError(statusCode int, err error) *Error {
	return &Error{
		StatusCode: statusCode,
		Err:        err,
	}
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) HTTPStatusCode() int {
	return e.StatusCode
}

func (e *Error) Unwrap() error {
	return e.Err
}
