# Design

## Context

Greenfield Go module `github.com/matbur/stargo` on Go 1.27, with the Python Stario implementation available as git-ignored reference at `stario/`. Requirements live in `specs/`; motivation in `proposal.md`.

Constraints that shape this design:

- No custom HTTP server. The reference implementation spends roughly a third of its code on a compiled HTTP/1.1 + HTTP/2 stack, and Go already provides that.
- Minimal surface: no Datastar attribute/action helper layer, no static asset pipeline, no form handling, no telemetry UI.
- One HTML component library as the primary view layer, reached through structural interfaces so the core stays library-agnostic.
- Live pages follow the reference model: one long-lived SSE response owns the view, commands change state and publish, never render.

## Goals / Non-Goals

**Goals:**

- A request path with one obvious owner of the response, one error path, and a span available in every handler.
- An SSE package whose output bytes match the Datastar protocol exactly, with predictable flushing and no hidden buffering.
- A relay whose publish never blocks a request and whose subscribers coalesce per subject.
- Tracing that is off by default, cheap when off, and queryable with plain SQL tooling over NDJSON.
- Small dependency surface: standard library plus one HTML component library.

**Non-Goals:**

- Parity with Stario's Python API or its compiled server behavior.
- HTTP/2 tuning, SSE compression, multipart forms, cookies/session middleware, static asset fingerprinting, OpenTelemetry export.
- Cross-process pub/sub, durability, or replay.
- Zero-allocation or benchmark-driven optimization before measurement.

## Decisions

### D1: `net/http` for transport

Use `http.Server`, `http.ServeMux`, and `http.ResponseController`. No custom listener, no HTTP/2 implementation, no connection drain layer.

Alternatives: port the compiled server (rejected: an order of magnitude more code for behavior `net/http` already has); adopt a third-party router (rejected: Go 1.22 patterns cover `{id}` and method routing).

Consequence: live-stream behavior depends on server timeouts. `WriteTimeout` must be zero for streamed responses; this is documented and set in the example server.

### D2: `Handler func(*Ctx) error` with centralized error mapping

A handler returns an error; the application maps it to a status, marks the span, and decides between writing a response and aborting the connection.

Alternatives: `http.HandlerFunc` (rejected: every handler would re-implement tracing and SSE setup); `http.Handler` plus `context.Value` (rejected: hides the span and invites type assertions).

Consequence: `Ctx` is a small struct, owned per request, with `W`, `R`, `Span`, and response state. No pooling initially.

### D3: Structural HTML inputs, no adapter, no library import

```go
type Renderer interface {
	Render(context.Context, io.Writer) error
}

type writerRenderer interface {
	Render(io.Writer) error
}
```

`HTML` and `PatchElements` accept `any` and resolve it by type switch: `Renderer`, `writerRenderer`, `string`, `[]byte`. Unknown types return an error.

Alternatives: a dedicated adapter package wrapping `gomponents.Node` (rejected: an extra import for every view file and one more package to document); a generics-based constraint (rejected: unions of interfaces with methods do not express this cleanly); depend on gomponents directly in the core (rejected: locks the API and violates the dependency rule).

Consequence: gomponents is used without an adapter, and the core imports nothing from it. The price is losing compile-time checking at the call site: an unsupported value fails at runtime with an error, not at build time. Tests cover each accepted form and the rejection case.

### D4: SSE stream is the only writer

```go
type Stream struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	rc      *http.ResponseController
	started bool
	closed  bool
	err     error
}
```

Each event is built in one `[]byte` buffer and written once, followed by `Flush`. The mutex covers the whole write path, so several goroutines cannot interleave event bytes.

Alternatives: one writer goroutine per stream with a channel (rejected: more moving parts, harder to propagate errors to callers); no lock, documented single-goroutine use (rejected: too easy to misuse from a second goroutine).

Encoding rules mirror the protocol: one `data:` line when the payload has no CR/LF, split lines otherwise; `retry: 1000` omitted; NUL rejected in event ids; line breaks rejected in selectors.

Consequence: no compression on streams, by design. A gzip wrapper flushing per event costs CPU and gains little on short events.

### D5: Relay with per-subject coalescing, drop-oldest

```go
type Relay[T any]

func (r *Relay[T]) Publish(subject string, v T)
func (r *Relay[T]) Subscribe(ctx context.Context, pattern string, opts ...Option) (*Subscription[T], error)

type Subscription[T] struct {
	mu       sync.Mutex
	pending  map[string]entry[T] // subject -> {value, seq}
	maxSize  int
	dropped  uint64
	wake     chan struct{} // cap 1
}
```

`Publish` snapshots matching subscriptions under a read lock, then updates each subscription's pending map under its own lock, then performs a non-blocking send on a one-slot wake channel.

Alternatives: buffered channel per subscription (rejected: loses per-subject coalescing and lets a slow tab replay a backlog); blocking publish (rejected: one stuck tab would stall a command); trie index (deferred: no measurement justifying it).

Consequence: `Latest()` is expressed as a limit of one pending value per subject; `MaxSize(n)` bounds the number of pending subjects and drops the oldest. Drops are counted, never silent.

### D6: Trace tree via `parent_id`, NDJSON sink

Spans carry `span_id`, `trace_id`, and `parent_id`; no child slices, no registry. Export is one JSON object per finished span with nested `events`.

`Step` returns a child span and must be ended; a scoped helper (`Do`) closes it even on panic. Records may appear in the file in completion order, so readers group by `trace_id`.

Alternatives: in-memory trace assembly (rejected: memory grows with request concurrency, and a crash loses everything); SQLite sink (rejected: the reference project deprecates its own SQLite tracer in favor of NDJSON plus a SQL reader; NDJSON removes a driver, migrations, and write locking).

Writer: bounded channel, one goroutine, batch by count or interval, `bufio.Writer`, optional fsync, optional size rotation, drop-plus-counter under pressure, drain on close.

Consequence: stream spans need `kind: stream`; otherwise hours-long connections distort duration statistics.

### D7: `uuid` from the standard library

Go 1.27 ships `uuid` (RFC 9562). Span and trace ids use `uuid.NewV7()`: time-ordered, so ids sort usefully and JSON encoding uses `MarshalText`.

Alternatives: `github.com/google/uuid` (rejected: redundant on 1.27); hand-rolled randomness (rejected: reinvents a stdlib package).

### D8: Datastar attributes stay plain HTML

No helper layer for `data-on:click`, `data-bind`, `data-show`, and friends. Views call the HTML library's `Data`-style helper or pass attributes directly.

Alternatives: a mirrored `data.*` / `at.*` API (rejected: roughly a thousand lines to reproduce a string-formatted attribute namespace).

Consequence: attribute names become strings in views. Typos are caught by tests, not the compiler. Accepted for v0.

### D9: Route handles as values, not code generation

Routes are immutable values declared at package level:

```go
var ROOM = route.Get("/rooms/{id}")
```

The same value feeds registration (`app.Handle(ROOM, handler)`) and URL building (`ROOM.Href(id)`), and its pattern becomes the span name.

Alternatives: strings only (rejected: pattern duplication between registration and links, and no place to attach URL building); code generation per route, mirroring the reference implementation's compiled `href` function (rejected: build step for behavior a positional variadic reproduces); typed generics per placeholder count (rejected: path values are strings, so generics only enforce arity, at the cost of noisy declarations).

Consequence: `Href` panics on arity mismatch or on values for a static route. These are programmer errors, and the application recover path turns them into `500` plus a panic event instead of a process exit. Host-qualified routes are not supported, because the standard mux does not route on host.

### D10: Fixed package layout

```
stargo/            App, Ctx, Handler, responses, errors
  route/           Route handle, URL building
  sse/             Stream and event encoding
  relay/           Relay[T], Subscription[T]
  trace/           Span, tracer, NDJSON writer
  signals/         Datastar signal reader
  examples/hello/  runnable example
```

Alternatives: one flat package (rejected: SSE and trace have no reason to share a namespace); internal-first layout (rejected: these are the public API).

### D12: Framework-owned lifecycle

```go
func (a *App) ListenAndServe(addr string, opts ...ServeOption) error
func (a *App) Serve(ctx context.Context, ln net.Listener, opts ...ServeOption) error
```

`Serve` takes a listener and a root context and installs no signal handling, so tests and unix sockets use it directly. `ListenAndServe` adds signal handling and an address.

Shutdown order is fixed: cancel the root context, then `Server.Shutdown` with a timeout, then close the tracer. The order exists because `http.Server.Shutdown` stops accepting connections and waits for handlers, but does not cancel their contexts; a live SSE handler would wait forever. `Server.BaseContext` returns the application root context, so every request context is its descendant and cancellation reaches handlers first.

Server defaults: `ReadHeaderTimeout` 5s, `WriteTimeout` 0, `IdleTimeout` 120s, `ErrorLog` routed to `slog`, shutdown timeout 10s. Options override the shutdown timeout, the read-header timeout, and expose the raw `*http.Server` for anything else.

Alternatives: leave `http.Server` assembly to the application (rejected: every app repeats the same shutdown bug, and SSE makes the bug certain); kill handlers after the timeout (rejected: forced connection cuts lose buffered writes and hide the real problem); install signal handling inside `Serve` (rejected: makes it unusable in tests and nested servers).

Consequence: a handler that ignores its context is cut off when the shutdown timeout expires, and the entry point returns `context.DeadlineExceeded`. That is deliberate: a hung stream must not block a deploy.

### D13: Tests through `httptest` and byte assertions

Handlers are tested with `httptest.NewRecorder`; SSE tests compare exact event bytes with a recorder that implements `Flush`. Trace tests assert parsed JSON lines, not string prefixes.

Alternatives: golden files for whole pages (deferred: noisy early on); a custom test client like the reference implementation (rejected: stdlib is enough at this size).

## Risks / Trade-offs

- **SSE behind buffering proxies** → document `X-Accel-Buffering: no` and require an immediate flush after headers; example nginx config in docs.
- **`http.Server.Shutdown` waits for live handlers** → provide a root context cancelled before shutdown and document a bounded shutdown timeout; long streams end through context cancellation, not by timeout killers.
- **Long-lived stream spans distort latency queries** → `kind` field plus documented queries that filter it out.
- **NDJSON grows without bound** → rotation opt-in and documented prune; stdout destination for container deployments.
- **Relay drops under load** → counters exposed through a stats accessor; drop policy documented as at-most-once, newest-wins per subject.
- **Mutex-per-stream serializes multi-goroutine producers** → acceptable for page patches; document that fan-out belongs to one rendering goroutine per connection.
- **gomponents attribute names are unescaped** → convention: never build element or attribute names from user input; covered in the html-rendering spec.
- **Tracing cost is invisible when off** → no-op span type with empty methods; a benchmark task verifies the off path allocates nothing in the request path beyond the span struct.
- **`Href` panics on programmer error** → the application recovery path turns it into `500` plus a panic event; tests cover wrong arity and values on a static route.
- **Handler ignores context on shutdown** → bounded shutdown timeout returns a deadline error; documented as a handler contract, not worked around silently.

## Migration Plan

Greenfield. No migration of existing data or APIs.

Order of work: core module and HTTP layer, then rendering, then SSE, then relay, then signals, then tracing, then the example app, then docs. Each package lands with tests before the next depends on it.

Rollback: delete the module. Nothing outside `stargo/` depends on it.

## Open Questions

- Whether generated or hand-written route helpers (`href()` style) are worth adding after the first real app — deferred, no spec impact.
- Whether `Ctx` should be pooled — deferred until a profile shows request allocation as a cost.
