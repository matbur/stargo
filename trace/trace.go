// Package trace's public surface: tracers, options, and environment setup.
package trace

import (
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Tracer creates spans and owns the export writer.
//
// A nil *Tracer is valid and does nothing, which is how disabled tracing is
// represented.
type Tracer struct {
	writer *writer
	closed atomic.Bool
}

// Option configures a Tracer.
//
// The option type carries an unexported configuration struct, so options are
// created with the constructors in this package.
type Option func(*config)

type config struct {
	path          string
	destination   io.Writer
	queueSize     int
	batchSize     int
	flushInterval time.Duration
	rotateBytes   int64
	rotateKeep    int
	syncWrites    bool
}

func defaults() config {
	return config{
		queueSize:     65536,
		batchSize:     512,
		flushInterval: 125 * time.Millisecond,
		rotateKeep:    5,
	}
}

// File writes spans to path, appending to an existing file.
func File(path string) Option {
	return func(c *config) {
		c.path = path
		c.destination = nil
	}
}

// Stdout writes spans to standard output.
func Stdout() Option {
	return func(c *config) {
		c.destination = os.Stdout
		c.path = ""
	}
}

// Destination writes spans to an arbitrary writer. Rotation is not available
// for a custom destination.
func Destination(w io.Writer) Option {
	return func(c *config) {
		c.destination = w
		c.path = ""
	}
}

// Rotate starts a new file when maxBytes have been written and keeps up to
// keep total files including the current one. Keep must be at least 1.
func Rotate(maxBytes int64, keep int) Option {
	return func(c *config) {
		if maxBytes > 0 {
			c.rotateBytes = maxBytes
		}
		if keep > 1 {
			c.rotateKeep = keep
		}
	}
}

// QueueSize bounds the number of spans waiting for the writer.
func QueueSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.queueSize = n
		}
	}
}

// BatchSize sets the number of pending spans that triggers a flush.
func BatchSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

// FlushInterval sets the maximum time a span waits before being written.
func FlushInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.flushInterval = d
		}
	}
}

// SyncWrites makes the writer call Sync after every flush.
func SyncWrites() Option {
	return func(c *config) {
		c.syncWrites = true
	}
}

// New returns a Tracer writing to the configured destination.
//
// With no destination the tracer writes to trace.ndjson in the working
// directory. Use FromEnv to keep tracing off unless configured.
func New(opts ...Option) *Tracer {
	cfg := defaults()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.destination == nil && cfg.path == "" {
		cfg.path = "trace.ndjson"
	}
	return &Tracer{writer: newWriter(cfg)}
}

// FromEnv builds a tracer from environment variables.
//
// TRACE disables tracing when unset, empty, "0", "off", or "false". The value
// "-" writes to standard output. The value "1", "true", or "on" enables a file
// destination taken from TRACE_FILE, defaulting to trace.ndjson. Any other
// value is used as the file path. TRACE_MAX_MB and TRACE_KEEP opt into size
// rotation.
func FromEnv() *Tracer {
	value := strings.TrimSpace(os.Getenv("TRACE"))
	switch strings.ToLower(value) {
	case "", "0", "off", "false":
		return nil
	case "-":
		return New(Stdout())
	}

	path := value
	switch strings.ToLower(value) {
	case "1", "true", "on":
		path = strings.TrimSpace(os.Getenv("TRACE_FILE"))
		if path == "" {
			path = "trace.ndjson"
		}
	}

	opts := []Option{File(path)}
	if mb, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("TRACE_MAX_MB")), 10, 64); err == nil && mb > 0 {
		keep := 5
		if parsed, err := strconv.Atoi(strings.TrimSpace(os.Getenv("TRACE_KEEP"))); err == nil && parsed > 1 {
			keep = parsed
		}
		opts = append(opts, Rotate(mb<<20, keep))
	}
	return New(opts...)
}

// Start opens a root span with its own trace id.
//
// A nil tracer, or a tracer that has been closed, returns nil. Span methods on
// a nil span are no-ops.
func (t *Tracer) Start(name, kind string) *Span {
	if t == nil || t.writer == nil || t.closed.Load() {
		return nil
	}
	return t.start(name, kind)
}

// Close stops accepting spans, drains the queue, flushes, and closes the
// destination. It is safe to call more than once.
func (t *Tracer) Close() error {
	if t == nil || t.writer == nil {
		return nil
	}
	if !t.closed.CompareAndSwap(false, true) {
		return nil
	}
	return t.writer.close()
}

// DroppedSpans returns the number of spans lost to a full queue or an encoding
// failure.
func (t *Tracer) DroppedSpans() uint64 {
	if t == nil || t.writer == nil {
		return 0
	}
	return t.writer.dropped.Load()
}
