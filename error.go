package ctxwire

import (
	"fmt"
)

// Error is the error type used by the package.
// It wraps the original error and adds a message.
type Error struct {
	message string
	err     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.err.Error())
}

// Unwrap implements the errors.Wrapper interface.
func (e *Error) Unwrap() error {
	return e.err
}

func newError(message string, err error) *Error {
	return &Error{message: message, err: err}
}
