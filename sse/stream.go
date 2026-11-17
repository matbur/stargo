package sse

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// Stream owns one Server-Sent Events response.
//
// Every event is encoded into one buffer, written once, and flushed before the
// call returns. The whole write path is serialized by a mutex, so several
// goroutines may write without interleaving event bytes.
type Stream struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	rc      *http.ResponseController
	ctx     context.Context
	started bool
	closed  bool
	err     error
}

// StreamOption configures a Stream.
type StreamOption func(*Stream)

// WithContext sets the request context passed to context-aware renderers.
func WithContext(ctx context.Context) StreamOption {
	return func(s *Stream) {
		if ctx != nil {
			s.ctx = ctx
		}
	}
}

// New returns a stream writing to w. Headers are sent on the first event.
func New(w http.ResponseWriter, opts ...StreamOption) *Stream {
	s := &Stream{
		w:   w,
		rc:  http.NewResponseController(w),
		ctx: context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Started reports whether the response headers have been sent.
func (s *Stream) Started() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// Closed reports whether Close was called or the stream disconnected.
func (s *Stream) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Err returns the write error that marked the stream disconnected, if any.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close stops the stream. Further writes return an error. Closing a stream
// that is already closed is harmless.
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.err != nil {
		return s.err
	}
	return nil
}

var (
	errClosed      = errors.New("sse: stream is closed")
	errNilWriter   = errors.New("sse: stream has no response writer")
	errNotEventSSE = errors.New("sse: response already started without Content-Type: text/event-stream")
)

// prepareLocked validates the stream state and sends headers on first use.
func (s *Stream) prepareLocked() error {
	if s.closed {
		if s.err != nil {
			return s.err
		}
		return errClosed
	}
	if s.w == nil {
		return errNilWriter
	}
	if s.started {
		contentType := s.w.Header().Get("Content-Type")
		if contentType != "" && contentType != contentTypeEventStream {
			return errNotEventSSE
		}
		return nil
	}

	s.w.Header().Set("Content-Type", contentTypeEventStream)
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(http.StatusOK)
	s.started = true
	return nil
}

// writeLocked writes one encoded event and flushes it. A failed write marks
// the stream disconnected.
func (s *Stream) writeLocked(event []byte) error {
	if err := s.prepareLocked(); err != nil {
		return err
	}
	if _, err := s.w.Write(event); err != nil {
		s.closed = true
		s.err = err
		return err
	}
	if err := s.rc.Flush(); err != nil {
		s.closed = true
		s.err = err
		return err
	}
	return nil
}

const contentTypeEventStream = "text/event-stream"
