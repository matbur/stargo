// Package trace's public surface: tracers, options, and environment setup.
package trace

import (
	"errors"
	"io"
	"net/url"
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
	sinks   []sink
	sampler Sampler
	closed  atomic.Bool
}

// Option configures a Tracer.
//
// The option type carries an unexported configuration struct, so options are
// created with the constructors in this package.
type Option func(*config)

type target struct {
	path        string
	destination io.Writer
}

type config struct {
	targets       []target
	otlp          *otlpConfig
	sampler       Sampler
	serviceName   string
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
		sampler:       ParentBased(AlwaysOn()),
	}
}

// File writes spans to path, appending to an existing file. Options that add a
// destination accumulate, so a tracer can write to several destinations.
func File(path string) Option {
	return func(c *config) {
		c.targets = append(c.targets, target{path: path})
	}
}

// Stdout writes spans to standard output.
func Stdout() Option {
	return func(c *config) {
		c.targets = append(c.targets, target{destination: os.Stdout})
	}
}

// Destination writes spans to an arbitrary writer. Rotation is not available
// for a custom destination.
func Destination(w io.Writer) Option {
	return func(c *config) {
		c.targets = append(c.targets, target{destination: w})
	}
}

// ServiceName sets the OpenTelemetry service.name resource attribute. An empty
// name is ignored, so a name from the environment survives.
func ServiceName(name string) Option {
	return func(c *config) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			c.serviceName = trimmed
		}
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
	if len(cfg.targets) == 0 && cfg.otlp == nil {
		cfg.targets = []target{{path: "trace.ndjson"}}
	}
	return buildTracer(cfg)
}

func buildTracer(cfg config) *Tracer {
	sampler := cfg.sampler
	if sampler == nil {
		sampler = ParentBased(AlwaysOn())
	}
	tracer := &Tracer{sinks: make([]sink, 0, len(cfg.targets)+1), sampler: sampler}
	for _, destination := range cfg.targets {
		tracer.sinks = append(tracer.sinks, newWriter(cfg, destination))
	}
	if cfg.otlp != nil {
		tracer.sinks = append(tracer.sinks, newOTLPSink(*cfg.otlp, cfg, cfg.serviceName))
	}
	return tracer
}

// SetServiceName updates the service name used by remote exporters. It has no
// effect on local NDJSON output.
func (t *Tracer) SetServiceName(name string) {
	if t == nil {
		return
	}
	for _, sink := range t.sinks {
		if setter, ok := sink.(interface{ setServiceName(string) }); ok {
			setter.setServiceName(name)
		}
	}
}

// FromEnv builds a tracer from environment variables, then applies opts on top
// so explicit options win.
//
// TRACE disables tracing when "0", "off", or "false". The value "-" writes to
// standard output. The value "1", "true", or "on" enables a file destination
// taken from TRACE_FILE, defaulting to trace.ndjson. Any other value is used as
// the file path. TRACE_MAX_MB and TRACE_KEEP opt into size rotation.
//
// An OTLP endpoint in OTEL_EXPORTER_OTLP_TRACES_ENDPOINT or
// OTEL_EXPORTER_OTLP_ENDPOINT enables tracing even when TRACE is unset. The
// exporter also reads OTEL_EXPORTER_OTLP_HEADERS, OTEL_SERVICE_NAME,
// OTEL_TRACES_SAMPLER, and OTEL_TRACES_SAMPLER_ARG. FromEnv returns nil when no
// destination is configured.
func FromEnv(opts ...Option) *Tracer {
	cfg := defaults()

	if endpoint, exact, ok := otlpEndpointFromEnv(); ok {
		otlp := ensureOTLP(&cfg)
		otlp.endpoint = endpoint
		otlp.exact = exact
		otlp.headers = parseOTLPHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	}
	if name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); name != "" {
		cfg.serviceName = name
	}
	if sampler := samplerFromEnv(); sampler != nil {
		cfg.sampler = sampler
	}

	value := strings.TrimSpace(os.Getenv("TRACE"))
	switch strings.ToLower(value) {
	case "0", "off", "false":
		return nil
	case "":
		// No local destination; an OTLP endpoint or an explicit option may
		// still enable tracing below.
	case "-":
		cfg.targets = append(cfg.targets, target{destination: os.Stdout})
	default:
		path := value
		switch strings.ToLower(value) {
		case "1", "true", "on":
			path = strings.TrimSpace(os.Getenv("TRACE_FILE"))
			if path == "" {
				path = "trace.ndjson"
			}
		}
		cfg.targets = append(cfg.targets, target{path: path})
		if mb, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("TRACE_MAX_MB")), 10, 64); err == nil && mb > 0 {
			cfg.rotateBytes = mb << 20
			if keep, err := strconv.Atoi(strings.TrimSpace(os.Getenv("TRACE_KEEP"))); err == nil && keep > 1 {
				cfg.rotateKeep = keep
			}
		}
	}

	for _, opt := range opts {
		opt(&cfg)
	}
	if len(cfg.targets) == 0 && cfg.otlp == nil {
		return nil
	}
	return buildTracer(cfg)
}

func otlpEndpointFromEnv() (endpoint string, exact bool, ok bool) {
	if value := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); value != "" {
		return value, true, true
	}
	if value := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); value != "" {
		return value, false, true
	}
	return "", false, false
}

func parseOTLPHeaders(value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	headers := make(map[string]string)
	for _, pair := range strings.Split(value, ",") {
		key, raw, found := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			continue
		}
		decoded, err := url.PathUnescape(strings.TrimSpace(raw))
		if err != nil {
			decoded = strings.TrimSpace(raw)
		}
		headers[key] = decoded
	}
	return headers
}

// Start opens a root span with its own trace id.
//
// A nil tracer, or a tracer that has been closed, returns nil. Span methods on
// a nil span are no-ops.
func (t *Tracer) Start(name, kind string) *Span {
	if t == nil || len(t.sinks) == 0 || t.closed.Load() {
		return nil
	}
	return t.start(name, kind)
}

// Close stops accepting spans, drains the queues, flushes, and closes every
// destination. It is safe to call more than once.
func (t *Tracer) Close() error {
	if t == nil || len(t.sinks) == 0 {
		return nil
	}
	if !t.closed.CompareAndSwap(false, true) {
		return nil
	}
	var errs []error
	for _, sink := range t.sinks {
		if err := sink.close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DroppedSpans returns the number of spans lost across all destinations.
func (t *Tracer) DroppedSpans() uint64 {
	if t == nil {
		return 0
	}
	var total uint64
	for _, sink := range t.sinks {
		total += sink.dropped()
	}
	return total
}
