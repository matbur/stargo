package stargo

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/matbur/stargo/internal/console"
	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/trace"
)

// Middleware wraps an http.Handler. Register middleware with Use before the
// server starts.
type Middleware func(http.Handler) http.Handler

// Option configures an App.
type Option func(*App)

// WithTracer sets the tracer. Passing nil disables tracing.
//
// Without this option an App uses the tracer described by the TRACE
// environment variables, which is nil when tracing is off.
func WithTracer(tracer *trace.Tracer) Option {
	return func(a *App) {
		a.tracer = tracer
		a.tracerSet = true
	}
}

// WithName sets the application name shown in console output. Without it the
// executable base name is used.
func WithName(name string) Option {
	return func(a *App) {
		a.name = strings.TrimSpace(name)
	}
}

// App is an HTTP application: a mux, middleware, and a tracer.
type App struct {
	mux        *http.ServeMux
	middleware []Middleware
	tracer     *trace.Tracer
	tracerSet  bool
	name       string
	rootCtx    context.Context
	cancel     context.CancelFunc
	console    atomic.Pointer[console.Writer]
	inflight   atomic.Int64

	mu    sync.Mutex
	chain http.Handler
}

// New returns an application with tracing configured from the environment.
//
// Options are applied before the environment tracer is built, so WithName can
// supply the OpenTelemetry service name. WithTracer replaces the environment
// tracer entirely.
func New(opts ...Option) *App {
	rootCtx, cancel := context.WithCancel(context.Background())
	app := &App{
		mux:     http.NewServeMux(),
		rootCtx: rootCtx,
		cancel:  cancel,
	}
	for _, opt := range opts {
		opt(app)
	}
	if !app.tracerSet {
		app.tracer = trace.FromEnv(trace.ServiceName(app.name))
	}
	return app
}

// Handle registers a handler for a route handle. The combined pattern is used
// both for dispatch and as the request span name.
func (a *App) Handle(routeHandle route.Route, handler Handler) {
	a.mux.Handle(routeHandle.Pattern(), a.wrap(routeHandle.Pattern(), handler))
}

// HandleFunc registers a handler for a raw net/http pattern. Use Handle with a
// route handle when the same path is also used to build links.
func (a *App) HandleFunc(pattern string, handler Handler) {
	a.mux.Handle(pattern, a.wrap(pattern, handler))
}

// HandleHTTP registers a standard http.Handler for a raw net/http pattern.
// The handler owns the response, while the framework records the request span,
// route attributes, and status and adopts incoming trace context.
func (a *App) HandleHTTP(pattern string, handler http.Handler) {
	if handler == nil {
		panic("stargo: handler is nil")
	}
	a.mux.Handle(pattern, a.wrapHTTP(pattern, handler))
}

// Use appends middleware. The first registered middleware is the outermost.
// Register middleware before serving requests.
func (a *App) Use(middleware Middleware) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.middleware = append(a.middleware, middleware)
	a.chain = nil
}

// Close cancels the application root context and drains the tracer.
func (a *App) Close() error {
	a.cancel()
	return a.tracer.Close()
}

// ServeHTTP implements http.Handler.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.inflight.Add(1)
	defer a.inflight.Add(-1)
	started := time.Now()

	a.mu.Lock()
	chain := a.chain
	if chain == nil {
		chain = a.buildChain()
		a.chain = chain
	}
	a.mu.Unlock()

	recorder := &statusRecorder{ResponseWriter: w}
	chain.ServeHTTP(recorder, r)

	// net/http sets Request.Pattern when the mux matches. A request with no
	// pattern was answered by the mux itself: 404 or 405.
	if r.Pattern == "" {
		status := recorder.status
		if status == 0 {
			status = http.StatusNotFound
		}
		span := a.requestSpan(r, "unmatched")
		span.Attr("http.method", r.Method)
		span.Attr("http.path", r.URL.Path)
		span.Attr("http.status", status)
		span.End()
	}

	if writer := a.console.Load(); writer != nil {
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		writer.Write(console.Block{
			Start:    started,
			Name:     r.Method + " " + r.URL.Path,
			Status:   status,
			Failed:   status >= 500,
			Duration: time.Since(started),
		})
	}
}

func (a *App) buildChain() http.Handler {
	handler := http.Handler(a.mux)
	for i := len(a.middleware) - 1; i >= 0; i-- {
		handler = a.middleware[i](handler)
	}
	return handler
}

func (a *App) wrap(pattern string, handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := a.requestSpan(r, pattern)
		r = r.WithContext(trace.ContextWithSpan(r.Context(), span))
		c := &Ctx{
			W:    w,
			R:    r,
			Span: span,
			app:  a,
		}
		c.Span.Attr("http.route", pattern)
		c.Span.Attr("http.method", r.Method)
		c.Span.Attr("http.path", r.URL.Path)
		defer c.Span.End()

		err, panicked := c.run(handler)
		if panicked != nil {
			c.handlePanic(panicked)
			return
		}
		if err != nil {
			c.handleError(err)
			return
		}
		c.finish()
	}
}

// requestSpan continues an incoming W3C trace when the request carries one and
// otherwise starts a new trace.
func (a *App) requestSpan(r *http.Request, name string) *trace.Span {
	remote, ok := trace.ExtractContext(
		r.Header.Get("traceparent"),
		r.Header.Get("tracestate"),
	)
	if ok {
		return a.tracer.StartRemote(name, "request", remote)
	}
	return a.tracer.Start(name, "request")
}

func (a *App) wrapHTTP(pattern string, handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := a.requestSpan(r, pattern)
		r = r.WithContext(trace.ContextWithSpan(r.Context(), span))
		span.Attr("http.route", pattern)
		span.Attr("http.method", r.Method)
		span.Attr("http.path", r.URL.Path)
		defer span.End()

		recorder := &statusRecorder{ResponseWriter: w}
		handler.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		span.Attr("http.status", status)
	}
}

// statusRecorder observes the status written by the mux for unmatched
// requests and exposes the underlying writer to http.ResponseController.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
