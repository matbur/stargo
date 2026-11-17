package stargo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/matbur/stargo/internal/render"
	"github.com/matbur/stargo/sse"
	"github.com/matbur/stargo/trace"
)

// Handler is the function registered for a route.
//
// Returning nil means the handler is finished. Returning an error maps to an
// HTTP error response, or aborts the connection when a response already
// started.
type Handler func(*Ctx) error

// Ctx is the request context passed to handlers.
//
// W, R, and Span are owned by the request. Use the response helpers rather
// than writing to W directly so the framework can track response state.
type Ctx struct {
	W    http.ResponseWriter
	R    *http.Request
	Span *trace.Span

	app *App

	mu       sync.Mutex
	started  bool
	complete bool
	streamed bool
	status   int
	stream   *sse.Stream
}

// HTML sends HTML. The value may be a context renderer, a writer renderer, a
// string, or a byte slice. Strings and bytes are trusted HTML and are not
// escaped.
func (c *Ctx) HTML(value any) error {
	body, err := render.HTML(c.R.Context(), value)
	if err != nil {
		return err
	}
	return c.respond(http.StatusOK, "text/html; charset=utf-8", body)
}

// JSON sends a JSON value.
func (c *Ctx) JSON(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.respond(http.StatusOK, "application/json", body)
}

// Text sends plain text.
func (c *Ctx) Text(text string) error {
	return c.respond(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
}

// NoContent sends 204 with no body.
func (c *Ctx) NoContent() error {
	return c.respond(http.StatusNoContent, "", nil)
}

// Redirect sends a redirect with an empty body.
func (c *Ctx) Redirect(status int, location string) error {
	if status < 300 || status > 399 {
		return fmt.Errorf("stargo: redirect status must be 3xx, got %d", status)
	}
	if location == "" {
		return fmt.Errorf("stargo: redirect location is empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started || c.complete {
		return errResponseStarted
	}
	c.started = true
	c.complete = true
	c.status = status
	c.W.Header().Set("Location", location)
	c.W.WriteHeader(status)
	return nil
}

// Stream starts a Server-Sent Events response and marks the request span kind
// as stream.
func (c *Ctx) Stream() (*sse.Stream, error) {
	c.mu.Lock()
	if c.started || c.complete {
		c.mu.Unlock()
		return nil, errResponseStarted
	}
	c.started = true
	c.streamed = true
	c.status = http.StatusOK
	c.mu.Unlock()

	c.Span.SetKind("stream")
	stream := sse.New(c.W, sse.WithContext(c.R.Context()))
	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()
	return stream, nil
}

// ResponseStarted reports whether the response has begun.
func (c *Ctx) ResponseStarted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// Status returns the response status, or zero when nothing was written.
func (c *Ctx) Status() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Ctx) respond(status int, contentType string, body []byte) error {
	c.mu.Lock()
	if c.started || c.complete {
		c.mu.Unlock()
		return errResponseStarted
	}
	c.started = true
	c.complete = true
	c.status = status
	if contentType != "" {
		c.W.Header().Set("Content-Type", contentType)
	}
	c.mu.Unlock()

	c.W.WriteHeader(status)
	if len(body) == 0 {
		return nil
	}
	_, err := c.W.Write(body)
	return err
}

func (c *Ctx) run(handler Handler) (err error, panicked any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = nil
			panicked = recovered
		}
	}()
	return handler(c), nil
}

func (c *Ctx) finish() {
	c.mu.Lock()
	stream := c.stream
	started := c.started
	status := c.status
	c.mu.Unlock()

	if stream != nil {
		clientLeft := stream.Err() != nil
		if !clientLeft && c.R.Context().Err() != nil && c.app != nil && c.app.rootCtx.Err() == nil {
			clientLeft = true
		}
		if clientLeft {
			c.Span.Attr("request.client_left", true)
		}
		_ = stream.Close()
	}

	if !started {
		_ = c.NoContent()
		status = http.StatusNoContent
	}
	if status == 0 {
		status = http.StatusOK
	}
	c.Span.Attr("http.status", status)
}

func (c *Ctx) handleError(err error) {
	status := http.StatusInternalServerError
	message := http.StatusText(status)

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Status
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		if httpErr.Message != "" {
			message = httpErr.Message
		} else {
			message = http.StatusText(status)
		}
	}

	c.Span.Fail(err)
	c.Span.Event("exception", map[string]any{"message": err.Error()})
	c.Span.Attr("http.status", status)

	c.mu.Lock()
	started := c.started
	c.mu.Unlock()
	if started {
		panic(http.ErrAbortHandler)
	}

	c.mu.Lock()
	c.started = true
	c.complete = true
	c.status = status
	c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.mu.Unlock()

	c.W.WriteHeader(status)
	_, _ = fmt.Fprintln(c.W, message)
}

func (c *Ctx) handlePanic(recovered any) {
	if recovered == http.ErrAbortHandler {
		panic(recovered)
	}

	err := fmt.Errorf("panic: %v", recovered)
	c.Span.Fail(err)
	c.Span.Event("panic", map[string]any{
		"value": fmt.Sprint(recovered),
		"stack": string(debug.Stack()),
	})
	c.Span.Attr("http.status", http.StatusInternalServerError)

	c.mu.Lock()
	started := c.started
	c.mu.Unlock()
	if started {
		panic(http.ErrAbortHandler)
	}

	c.mu.Lock()
	c.started = true
	c.complete = true
	c.status = http.StatusInternalServerError
	c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.mu.Unlock()

	c.W.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintln(c.W, http.StatusText(http.StatusInternalServerError))
}
