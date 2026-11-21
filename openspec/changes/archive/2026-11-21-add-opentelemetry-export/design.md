# Design

## Context

See `proposal.md` for motivation and `specs/tracing/spec.md` for the required
behaviour.

Current shape:

- `Tracer` owns one `writer`. `Span.End` marshals a `record` to JSON and
  enqueues the encoded line; the writer goroutine batches lines, rotates files,
  and counts drops.
- `record` carries UUID strings for span, trace, and parent ids, nanosecond
  times, a free-form kind, a string status, attributes, and events.
- `App.wrap` starts the request span with no parent, so an incoming trace
  context is ignored. There is no span in the request context.
- `FromEnv` enables tracing only when `TRACE` is set. The only dependencies are
  the standard library and `maragu.dev/gomponents`; `uuid` is a standard
  package in this Go version.
- Span methods already tolerate a nil receiver, so tracing stays optional.

Constraints: no new module dependency, no change to the NDJSON record shape,
and no behaviour change when tracing is off or when only the existing options
are used.

## Goals / Non-Goals

**Goals:**

- One tracer can feed a local NDJSON writer and an OTLP/HTTP collector at the
  same time.
- Incoming W3C context continues an existing trace; outgoing context can be
  injected from a handler.
- Sampling is configurable and inherited by child spans.
- Standard OTLP environment variables work without extra setup.

**Non-Goals:**

- OTLP/gRPC and protobuf encoding.
- The OpenTelemetry Go SDK, metrics, logs, or baggage.
- Tail sampling, collector-side processing, or a local agent.
- Request-body or response-body capture.
- Renaming attributes to newer OpenTelemetry semantic conventions.

## Decisions

### A sink interface replaces the single writer

Introduce an internal `sink` with `enqueue(record)`, `close() error`, and
`dropped() uint64`. `Tracer` holds a slice of sinks. The existing writer becomes
the NDJSON sink and keeps its queue, batching, rotation, and `SyncWrites`
behaviour; JSON marshalling moves from `Span.End` into the writer goroutine,
which removes allocation from the request path and keeps the record shape
identical.

The OTLP sink has its own bounded queue and goroutine, batches records, and
sends one request per batch. Sharing one queue would force both sinks to pay the
slowest sink's latency; separate queues keep a stalled collector from delaying
the local file.

Alternative considered: a single exporter interface receiving bytes. Rejected
because OTLP needs the structured record to build one payload per batch, and
re-encoding the NDJSON line would lose nothing but cost a decode.

### Span and trace ids keep the UUID model

Internally a span still uses a UUIDv7 span id and a UUIDv7 trace id, so the
public `ID`, `TraceID`, `Step`, and NDJSON behaviour do not change. For OTLP:

- the trace id is the 16 UUID bytes;
- the span id is the last 8 bytes of the UUID;
- a parent span id is converted the same way;
- a remote parent span id occupies the last 8 bytes of a UUID whose first 8
  bytes are zero.

The truncation is stable within OTLP, but an NDJSON `parent_id` for a remote
parent is not the same textual value the upstream service used. That is
acceptable because NDJSON is a local debugging format; the spec only requires
the OTLP ids to preserve the trace relationship. Storing a separate 8-byte id
type would be cleaner but would churn the public `Span.ID()` API for no
observable gain.

### OTLP JSON is hand-encoded

Use `encoding/json` with explicit structs for `resourceSpans`, `scopeSpans`,
`spans`, attribute key/value pairs, events, and status. This keeps the module
dependency-free and produces the wire format collectors accept with
`Content-Type: application/json`. Protobuf and gRPC are non-goals.

Attribute conversion is centralized:

- `string` → `stringValue`
- `bool` → `boolValue`
- signed and unsigned integers → `intValue` encoded as a JSON string
- floats → `doubleValue`
- slices → `arrayValue`
- maps → `kvlistValue`
- anything else, including errors and `fmt.Stringer`, → `stringValue` via
  `fmt.Sprint`

64-bit nanosecond values are encoded as JSON strings, as OTLP/JSON requires.

Kind mapping: `request` and `stream` become OTLP server spans, `startup` becomes
internal, and an unknown kind falls back to internal. A failed span uses status
code 2 with the recorded error message; an ok span leaves the status unset so
collectors do not display a misleading "Ok".

### Transport and retry live in one small HTTP exporter

The exporter posts to the resolved traces endpoint with a configurable timeout
and default headers. Retry applies to network errors and to 429, 500, 502, 503,
and 504, with a fixed small set of attempts and exponential backoff. Other
statuses are permanent failures. Every failed batch increments the drop counter
by its span count. Retry delays are configurable internally so tests do not
sleep.

The queue is bounded and non-blocking: when full, the new record is dropped and
counted, matching the existing writer policy. `Close` stops intake, sends the
remaining batch, and respects the same bounded drain as the writer.

### Endpoint resolution follows the OTLP specification

- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is used unchanged.
- `OTEL_EXPORTER_OTLP_ENDPOINT` is treated as a base URL; `/v1/traces` is
  appended when it has no path, and joined without duplicating a trailing slash
  when it has one.
- The default is `http://localhost:4318/v1/traces` only when OTLP is explicitly
  requested without an endpoint.

Headers come from `OTEL_EXPORTER_OTLP_HEADERS` as comma-separated `key=value`
pairs with percent-decoded values; explicit options replace them entirely.

### Environment seeds the configuration, options override it

`FromEnv` builds a configuration from `TRACE`, the OTLP endpoint variables, the
sampler variables, and the service name, then applies any options passed to
`FromEnv`. `New` behaves the same way without environment discovery so tests and
programmatic use stay deterministic.

An OTLP endpoint set in the environment enables tracing even when `TRACE` is
unset, because there is no point reading a collector endpoint that would never
be used. `TRACE` still controls the file and stdout sinks. When both are
configured, both sinks run.

`App.New` changes its construction order: it applies options first and builds
the environment tracer afterwards, unless `WithTracer` was supplied. This lets
`WithName` reach the OTLP resource as `service.name` while keeping
`WithTracer(nil)` as the explicit "off" switch. Precedence is explicit OTLP
option, then `stargo.WithName`, then `OTEL_SERVICE_NAME`, then `stargo`.

### Propagation is part of the framework, injection is a helper

The request path parses `traceparent` before starting the request span. A valid
header supplies the trace id, the parent span id, the sampled flag, and an
optional `tracestate`; an invalid header is ignored rather than rejected, so a
bad client header cannot fail a request.

The active span is stored in the request context. `trace.Inject` reads it and
writes `traceparent` and `tracestate` into an `http.Header`; with no active span
it writes nothing. Handlers therefore do not depend on a framework-specific
client. Both helpers are exported and documented.

Validation follows W3C trace context: two hex digits for the version other than
`ff`, 32 hex digits for the trace id, 16 for the span id, nonzero ids, and two
hex digits for flags. Future versions with extra fields are accepted by reading
the first four fields.

### Sampling is decided once per root and inherited

A `Sampler` decides the root span. Child spans inherit the parent's decision; a
remote parent supplies its own decision, and the parent-based sampler respects
it. Non-sampled spans are not enqueued and skip attribute and event recording,
but keep their ids so propagation still works. The default is parent-based
always-on, which preserves today's behaviour exactly.

The trace-id ratio uses the low 8 bytes of the trace id as a big-endian
`uint64` and the standard threshold calculation, so the same trace id always
gets the same decision. Sampler configuration uses
`OTEL_TRACES_SAMPLER`/`OTEL_TRACES_SAMPLER_ARG` names plus matching constructors.

### Attributes keep their existing names

`http.method`, `http.path`, `http.route`, and `http.status` stay as they are.
Renaming them to `http.request.method` and friends would break existing NDJSON
queries and the README examples for no functional gain; a collector can remap
attributes if needed. Resource attributes use the standard `service.*`,
`host.name`, and `process.runtime.*` keys.

## Risks / Trade-offs

- An unauthenticated client can supply an arbitrary `traceparent`, forcing a
  sampling decision or polluting a trace → standard server behaviour; document
  that upstream proxies should strip or set the header, and keep the sampler
  configurable.
- JSON payloads are larger than protobuf → acceptable at current request
  volumes; request gzip can be added later without changing the model.
- Truncated span ids in OTLP mean NDJSON and OTLP ids are not directly
  comparable for remote parents → document in `trace/doc.go`; trace ids remain
  identical.
- A slow or unreachable collector can delay `Close` up to the drain timeout →
  the bounded queue keeps handlers unaffected, and the timeout bounds shutdown.
- Non-sampled spans skip attribute recording, so a handler cannot inspect them
  beyond ids → sampling is an export policy, not a debugging switch; keep the
  default always-on.
- Environment-driven construction makes `App.New` behaviour depend on ambient
  variables → `WithTracer` remains an explicit override and tests use it.

## Migration Plan

No migration is required. Existing callers of `trace.New`, `trace.FromEnv`, and
`stargo.New` keep their behaviour: tracing stays off unless `TRACE` or an OTLP
endpoint is set, and NDJSON records are unchanged. Rollout is per deployment:
set the OTLP endpoint and optionally the service name, then verify the collector
receives spans. Rollback is unsetting the endpoint variables; the local file
destination is unaffected.

## Open Questions

None. gRPC export, request compression, and attribute renaming are deliberate
non-goals that can become separate changes if needed.
