package sse

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

// Stream owns one Server-Sent Events response.
//
// Every event is encoded into one buffer, written once, and flushed before the
// call returns. The whole write path is serialized by a mutex, so several
// goroutines may write without interleaving event bytes.
//
// A stream created with WithRequest compresses events with brotli or gzip
// when the request accepts either. The compressor is flushed after every event.
type Stream struct {
	mu               sync.Mutex
	w                http.ResponseWriter
	rc               *http.ResponseController
	ctx              context.Context
	request          *http.Request
	encoding         string
	compressor       compressor
	compressorClosed bool
	started          bool
	closed           bool
	err              error
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
	if s.request != nil && s.request.Method != http.MethodHead {
		s.encoding = selectEncoding(s.request.Header.Get("Accept-Encoding"))
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
	if s.compressor != nil && !s.compressorClosed {
		s.compressorClosed = true
		if err := s.compressor.Close(); err != nil && s.err == nil {
			s.err = err
		}
	}
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

	header := s.w.Header()
	if s.encoding != "" && header.Get("Content-Encoding") == "" &&
		!hasDirective(header.Get("Cache-Control"), "no-transform") {
		s.compressor = newCompressor(s.encoding, s.w)
		header.Set("Content-Encoding", s.encoding)
		addVary(header)
	}
	header.Set("Content-Type", contentTypeEventStream)
	ensureNoCache(header)
	header.Set("X-Accel-Buffering", "no")
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
	writer := io.Writer(s.w)
	if s.compressor != nil {
		writer = s.compressor
	}
	if _, err := writer.Write(event); err != nil {
		return s.failLocked(err)
	}
	if s.compressor != nil {
		if err := s.compressor.Flush(); err != nil {
			return s.failLocked(err)
		}
	}
	if err := s.rc.Flush(); err != nil {
		return s.failLocked(err)
	}
	return nil
}

func (s *Stream) failLocked(err error) error {
	s.closed = true
	s.err = err
	return err
}

const contentTypeEventStream = "text/event-stream"
