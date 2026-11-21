# Tasks

## 1. Sink seam

- [x] 1.1 Replace the tracer's single writer with an internal sink interface (`enqueue`, `close`, `dropped`) and move NDJSON marshalling into the writer goroutine; verify `go test ./trace` keeps every existing NDJSON assertion passing.
- [x] 1.2 Let a tracer own several sinks and sum their drop counters; add a test with two NDJSON destinations where both files receive the same span.
- [x] 1.3 Preserve the default file destination in `New` and the off-by-default behaviour of `FromEnv`; verify the existing configuration tests still pass.

## 2. OTLP encoding

- [x] 2.1 Add OTLP JSON structures and id conversion: full trace id, last 8 bytes for span and parent ids; test hex lengths and that a child payload points at its parent.
- [x] 2.2 Convert span attributes to OTLP key/value pairs for strings, booleans, signed and unsigned integers, floats, slices, maps, and fallback values; test each JSON representation, including 64-bit integers encoded as strings.
- [x] 2.3 Map span kind, error status, events, resource attributes, and scope metadata into the payload; test a request span, a stream span, a startup span, and a failed span with an event.

## 3. OTLP transport

- [x] 3.1 Add an HTTP exporter with its own bounded queue, batching, and goroutine, plus endpoint resolution for base and traces URLs; test the request path, `Content-Type`, and decoded payload against `httptest`.
- [x] 3.2 Retry network errors and retryable statuses with bounded backoff and treat other statuses as permanent; test a 503 followed by 200 and a 400 that is not retried, using a short injected delay.
- [x] 3.3 Count undelivered spans as dropped without blocking the producer, and drain the last batch on close; test a full queue and a close with pending spans.
- [x] 3.4 Wire `trace.OTLP`, OTLP header and timeout options into `New`; test that a span reaches an `httptest` collector while an NDJSON file also receives it.

## 4. Sampling

- [x] 4.1 Add always-on, always-off, trace-id ratio, and parent-based samplers; test ratio determinism at 0, 1, and an intermediate value.
- [x] 4.2 Apply the sampling decision to root spans, inherit it in child spans, skip export for non-sampled spans, and keep ids available; test an unsampled trace that exports nothing and a sampled child that follows its parent.
- [x] 4.3 Parse `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG`; test each supported name, the ratio argument, and an unknown name falling back to the default.

## 5. Propagation

- [x] 5.1 Parse and validate `traceparent` (version, hex length, nonzero ids, flags) and keep `tracestate`; test a valid header, malformed input, zero ids, and a future version with extra fields.
- [x] 5.2 Expose the active span through the request context and add `Inject`; test that a handler reads the span ids and writes matching `traceparent` and `tracestate`, and that inject writes nothing when tracing is off.
- [x] 5.3 Make the request span adopt a valid incoming context and name the remote span as its parent; test an application request with a `traceparent` header and assert the exported trace id and parent.
- [x] 5.4 Inherit the remote sampled flag; test that an incoming unsampled context exports no spans while the handler still sees the trace id.

## 6. Environment, service name, docs

- [x] 6.1 Seed endpoint, headers, service name, and sampler from the standard environment variables, then apply explicit options on top; test endpoint enablement without `TRACE`, base-path appending, exact traces endpoint, header parsing, and option precedence.
- [x] 6.2 Build the environment tracer after application options so `stargo.WithName` becomes `service.name`, while `WithTracer` still wins; test both cases with an `httptest` collector.
- [x] 6.3 Document OTLP destinations, environment variables, propagation, and sampling in `README.md`, update the `trace` package docs, and mark 5.1 done in `ROADMAP.md`; verify the documented names and defaults match the implemented options and tests.

## 7. Integration verification

- [x] 7.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./trace .`; confirm all are clean.
- [x] 7.2 Run `openspec validate --changes --strict`; confirm the change passes.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review.
