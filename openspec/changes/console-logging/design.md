# Design

See `proposal.md` for motivation and `specs/console-logging/spec.md` for the behavior contract.

## Context

Stargo already has a trace span model with request attributes and an NDJSON writer, but it is off by default and has no console renderer. `Serve` owns the server lifecycle, while `ServeHTTP` owns request dispatch and already wraps responses in a `statusRecorder`, so both have the data console output needs without involving the tracer.

## Goals / Non-Goals

**Goals:**
- Produce Stario-shaped blocks for startup, one line per request, and shutdown.
- Keep console output on by default in an interactive terminal and silent otherwise.
- Reuse the request status already recorded by `statusRecorder`.
- Keep the renderer pure and unit-testable, separate from I/O and server lifecycle.

**Non-Goals:**
- Reproducing Stario's live footer, spinner, synchronized-output escapes, or terminal reflow handling.
- Exactly matching Stario's attribute set; Stargo reports only settings it actually has.
- Changing the tracing API, span record shape, or NDJSON format.
- Logging requests served outside `Serve`/`ListenAndServe`, such as `httptest.NewServer` in tests.

## Decisions

### Console is independent of tracing
The console renderer consumes small value types filled by `app.go` and `server.go`, not trace records. Alternatives: adding a second tracer sink or rendering from NDJSON records would couple console output to tracing being enabled and force a writer refactor. Independence also lets console output stay available when `TRACE` is off, which is the default.

### Renderer in `internal/console`
A pure formatter builds a block from `name`, start time, duration, status, attributes, and optional event. A writer serializes blocks behind one mutex and issues one `Write` per block, so concurrent requests cannot interleave lines. Rendering rules mirror Stario: width `min(COLUMNS-1, 100)` with a default of 100, fold the first dotted segment of the span name from attribute keys, group two or more keys sharing a dotted label, quote values containing spaces, and format durations by magnitude.

Alternatives: formatting inline at each call site repeats grouping and alignment logic and cannot be tested in isolation.

### Activation and configuration
`STARGO_CONSOLE` accepts `auto` (default), `1`, and `0`. `auto` checks whether standard output is a character device. Serve options `Console(w io.Writer)` and `NoConsole()` override the environment, mainly for embedding and tests. The destination is resolved when `Serve` starts, once per run.

### Application name
An `App` gets an optional name through a new `WithName(string)` option. Without it the startup block uses `filepath.Base(os.Args[0])`.

### Request timing and status
`ServeHTTP` captures the start time before dispatch and writes the request line after the handler returns, including unmatched requests answered by the mux. The status comes from the existing `statusRecorder`, falling back to 200 when nothing was written. A status of 500 or above selects the failure marker.

SSE handlers are logged when `ServeHTTP` returns, so a live stream produces its line on disconnect or shutdown with the full duration. This matches Stario, where the request span also ends with the stream.

### Shutdown accounting
`Serve` installs an `http.Server.ConnState` hook before serving. It counts open connections and classifies transitions during shutdown: idle to closed as `idle_closed`, active to closed as `force_closed`. An in-flight counter in `ServeHTTP` supplies `cancelled_tasks` as the number of requests whose contexts were cancelled at shutdown start. Connections still open after `Shutdown` returns are reported as `stale_connections` and counted again as `aborted` when `Close` drops them. The trigger is `expected_stop` for context cancellation and signals, and `runtime_failure` when serving fails.

On shutdown timeout the summary is written before `Serve` returns the deadline error, so the counters explain the failure.

### Styling
ANSI dim and bold are applied only when the destination is a terminal and `NO_COLOR` is unset. Colors carry no content, so tests compare plain text.

## Risks / Trade-offs

- [Connection counters do not map exactly to Stario's transport-level counters] → Document them as Stargo's connection states; keep names stable and values honest.
- [Startup duration measures only the gap between `Serve` entry and the serving goroutine start] → Name it startup and keep the measurement cheap; it stays in the same order of magnitude as Stario's banner.
- [A terminal-detected default can surprise a server run from an interactive shell with output captured elsewhere] → `STARGO_CONSOLE=0` and `NoConsole()` provide a direct escape hatch.
- [One line per request can be noisy for high-traffic processes] → Console output is off when stdout is redirected, so production deployments stay silent unless they opt in.
