# Proposal

## Why

Stario is a Python framework built around a clear mental model: HTTP in, HTML out, one long-lived SSE response per live page, and commands that change state without rendering. That model is stack-agnostic, but the Python implementation carries a large custom HTTP server and a markup engine that Go does not need. This change starts `github.com/matbur/stargo`, a Go framework that keeps the model and drops the machinery.

## What Changes

- New Go module `github.com/matbur/stargo`, targeting Go 1.27.
- HTTP layer on `net/http` only: `App` with `Handle(pattern, Handler)`, `Handler func(*Ctx) error`, centralized error mapping, panic recovery, request span wiring.
- Response helpers on `Ctx`: `HTML`, `JSON`, `Text`, `NoContent`, `Redirect`, `SSE`.
- `route` package: route handles declared once as values (`route.Get("/rooms/{id}")`) and used both for registration and for URL building, with path escaping, query, and fragment support.
- Application lifecycle owned by the framework: `ListenAndServe` and `Serve` start the server, cancel request contexts before shutdown so live streams end, drain tracing, and apply stream-safe timeouts.
- HTML views in pure Go with `maragu.dev/gomponents`; the core accepts HTML values structurally (context renderer, writer-only renderer, string, bytes), so gomponents is a choice, not a lock-in.
- Datastar SSE package: `PatchElements`, `PatchSignals`, `ExecuteScript`, `Remove`, `Navigate`, `Ping`. One event is one buffered write, then flush.
- In-process relay with per-subject coalescing (drop-oldest on overflow, non-blocking publish), supporting exact, `prefix.*`, and `*` subscriptions.
- Datastar request signals reader (`signals.Read`) for the `datastar` query parameter and JSON request bodies.
- NDJSON tracing: every request is a span, spans form a tree through `parent_id`, events are nested in the span record, writer batches and drops instead of blocking.
- No custom HTTP server, no HTTP/2 implementation, no SSE compression, no form/multipart handling, no static asset fingerprinting, no telemetry UI in this change.
- No Datastar attribute/action helper layer: `data-*` attributes are written directly in gomponents views.

## Capabilities

### New Capabilities
- `http-app`: application construction, route registration, handler contract, response helpers, error-to-status mapping, panic recovery, route handles and URL building, server lifecycle.
- `html-rendering`: the `Renderer` contract used by handlers and SSE, HTML escaping expectations, and how gomponents nodes are adapted.
- `sse-streams`: Datastar Server-Sent Events encoding, header and flush rules, event options, disconnect behavior.
- `relay`: in-process publish/subscribe with subject patterns, per-subject coalescing, bounded queues, and delivery guarantees.
- `request-signals`: reading Datastar signals from query parameters and request bodies, including size limits and untrusted-input rules.
- `tracing`: span creation, parent/child relationships, attributes, events, failure marking, NDJSON export, environment configuration.

### Modified Capabilities

None. The project has no existing specs.

## Impact

- New repository content: `go.mod`, root package `stargo`, `route/`, `sse/`, `relay/`, `trace/`, `signals/`, `examples/hello/`.
- New dependency: `maragu.dev/gomponents` (runtime). No cgo, no code generation, no build-time tooling.
- Reference code: the Python implementation of Stario is cloned at `stario/` and is git-ignored. It is a reference only; nothing imports it.
- Runtime configuration: `TRACE` and `TRACE_FILE` environment variables.
- Not affected: existing OpenSpec setup, `stario/` reference tree, `tmp/` notes.
