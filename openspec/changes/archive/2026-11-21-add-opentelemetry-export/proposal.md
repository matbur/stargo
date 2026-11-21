# Proposal

## Why

Stargo traces exist only as NDJSON on a local disk, so traces cannot reach an
OpenTelemetry collector, cannot be correlated across services, and cannot be
sampled. Any deployment that already runs Tempo, Jaeger, or Grafana has to
build its own bridge or lose the request tree the framework already records.

## What Changes

- Add an OTLP/HTTP JSON exporter with no new module dependency. Spans are
  batched, sent to a collector endpoint, retried on transient failures, and
  counted as dropped when delivery fails.
- Keep the existing NDJSON file, stdout, and arbitrary-writer destinations
  working, and allow an OTLP exporter and a local writer at the same time.
- Add W3C `traceparent` and `tracestate` propagation: the request span adopts
  an incoming trace, and handlers can inject the current context into outgoing
  requests.
- Add sampling: always-on, always-off, trace-id ratio, and parent-based
  variants, configured by option or by standard `OTEL_TRACES_SAMPLER`
  environment variables. Sampling affects export, not span creation, so IDs are
  still available for propagation.
- Add OpenTelemetry resource attributes: service name from `stargo.WithName`
  or `OTEL_SERVICE_NAME`, service version, host name, and runtime version.
- Add OTLP environment configuration (`OTEL_EXPORTER_OTLP_ENDPOINT`,
  `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`) and make
  an endpoint enable tracing from the environment.

No production dependency is added; the exporter uses the standard library.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `tracing`: adds OTLP export, W3C context propagation, sampling, OTLP resource
  attributes, and OTLP environment configuration on top of the existing span
  model, NDJSON export, and bounded writer.

## Impact

- `trace/`: new exporter seam, OTLP JSON encoding and transport, propagation,
  sampling, and resource attributes; existing NDJSON behaviour is preserved.
- `app.go`: request spans adopt incoming trace context, and the current span is
  available from the request context.
- `README.md` and `ROADMAP.md`: document the new destinations, environment
  variables, and propagation helpers.
- No new Go module dependencies.
- Existing tests keep passing; new tests cover encoding, transport, retry,
  sampling, and propagation against an `httptest` collector.
