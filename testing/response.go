package stargotest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// maxBodyInFailure is how much of a response body a failure message includes.
const maxBodyInFailure = 8 << 10

// Response is a buffered HTTP response.
type Response struct {
	tb TB

	// StatusCode is the response status code.
	StatusCode int
	// Header holds the response headers.
	Header http.Header
	// Body holds the complete response body.
	Body []byte
	// Request is the request that produced the response.
	Request *http.Request
}

// Text returns the body as a string.
func (r *Response) Text() string {
	return string(r.Body)
}

// DecodeJSON decodes the body into value.
func (r *Response) DecodeJSON(value any) {
	r.tb.Helper()

	if err := json.Unmarshal(r.Body, value); err != nil {
		r.fail("decode JSON: %v\nbody:\n%s", err, r.bodyForFailure())
	}
}

// AssertStatus fails the test unless the status code is want.
func (r *Response) AssertStatus(want int) *Response {
	r.tb.Helper()

	if r.StatusCode != want {
		r.fail("status = %d, want %d\nbody:\n%s", r.StatusCode, want, r.bodyForFailure())
	}
	return r
}

// AssertHeader fails the test unless the header value is want.
func (r *Response) AssertHeader(name, want string) *Response {
	r.tb.Helper()

	values := r.Header.Values(name)
	if len(values) == 0 {
		r.fail("header %q is missing, want %q", name, want)
		return r
	}
	if got := r.Header.Get(name); got != want {
		r.fail("header %q = %q, want %q", name, got, want)
	}
	return r
}

// AssertContains fails the test unless every fragment occurs in the body.
func (r *Response) AssertContains(fragments ...string) *Response {
	r.tb.Helper()

	body := string(r.Body)
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			r.fail("body does not contain %q\nbody:\n%s", fragment, r.bodyForFailure())
			return r
		}
	}
	return r
}

// AssertNotContains fails the test if any fragment occurs in the body.
func (r *Response) AssertNotContains(fragments ...string) *Response {
	r.tb.Helper()

	body := string(r.Body)
	for _, fragment := range fragments {
		if strings.Contains(body, fragment) {
			r.fail("body contains %q\nbody:\n%s", fragment, r.bodyForFailure())
			return r
		}
	}
	return r
}

// AssertCount fails the test unless fragment occurs want times.
func (r *Response) AssertCount(fragment string, want int) *Response {
	r.tb.Helper()

	if got := strings.Count(string(r.Body), fragment); got != want {
		r.fail("body contains %q %d times, want %d\nbody:\n%s", fragment, got, want, r.bodyForFailure())
	}
	return r
}

func (r *Response) fail(format string, args ...any) {
	prefix := "stargotest: "
	if r.Request != nil {
		prefix += r.Request.Method + " " + r.Request.URL.String() + ": "
	}
	r.tb.Fatalf(prefix+format, args...)
}

func (r *Response) bodyForFailure() string {
	if len(r.Body) <= maxBodyInFailure {
		return string(r.Body)
	}
	return string(r.Body[:maxBodyInFailure]) + fmt.Sprintf("\n... (%d bytes truncated)", len(r.Body)-maxBodyInFailure)
}
