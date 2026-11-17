package stargo

import (
	"errors"
	"net/http"
)

// HTTPError is a handler error carrying an HTTP status.
//
// Message is sent to the client. Wrap an internal error in Err to keep it out
// of the response while still tracing it.
type HTTPError struct {
	Status  int
	Message string
	Err     error
}

// NewHTTPError returns an HTTPError with a client-facing message.
func NewHTTPError(status int, message string) *HTTPError {
	return &HTTPError{Status: status, Message: message}
}

// Error implements error.
func (e *HTTPError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.Status)
}

// Unwrap returns the wrapped error.
func (e *HTTPError) Unwrap() error { return e.Err }

var errResponseStarted = errors.New("stargo: response already started")
