# Spec Delta

## ADDED Requirements

### Requirement: OTLP/HTTP JSON export
The tracer SHALL export finished spans to an OpenTelemetry collector over OTLP/HTTP using a JSON payload and the `application/json` content type.

#### Scenario: Span delivered to the collector
- **WHEN** tracing is configured with an OTLP endpoint and a request completes
- **THEN** the collector receives a POST to the traces path whose payload contains that span with hexadecimal trace, span, and parent ids, name, kind, start and end times in nanoseconds, attributes, and status

#### Scenario: Kind mapping
- **WHEN** a request span, a live stream span, or a startup span is exported
- **THEN** the request and stream spans use the OTLP server kind and the startup span uses the internal kind

#### Scenario: Error mapping
- **WHEN** a span is marked as failed
- **THEN** the exported span has an error status carrying the recorded error message

#### Scenario: Event mapping
- **WHEN** a span carries events
- **THEN** each event is exported with its name, nanosecond timestamp, and attributes

### Requirement: OTLP batching
The OTLP exporter SHALL batch finished spans and send them in one request per batch, flushing when the batch size or the flush interval is reached.

#### Scenario: Batch flush
- **WHEN** several spans finish within one flush interval
- **THEN** the collector receives them in a single request

#### Scenario: Shutdown flush
- **WHEN** the tracer is closed with spans still queued
- **THEN** the remaining spans are sent before shutdown completes

### Requirement: OTLP retry and drop accounting
The OTLP exporter SHALL retry transient delivery failures within a bounded number of attempts and SHALL count spans that cannot be delivered as dropped without blocking the handler that produced them.

#### Scenario: Transient failure then success
- **WHEN** the collector answers with a retryable status and later succeeds
- **THEN** the spans are retried and delivered, and the drop counter does not increase

#### Scenario: Permanent failure
- **WHEN** the collector rejects the payload with a non-retryable status
- **THEN** the spans are not retried and the drop counter increases by the number of spans in the batch

#### Scenario: Full queue
- **WHEN** spans are produced faster than they can be exported
- **THEN** excess spans are dropped, counted, and the request continues without waiting

### Requirement: Multiple export destinations
The tracer SHALL support an OTLP destination and a local NDJSON destination at the same time.

#### Scenario: Both destinations configured
- **WHEN** a span finishes while both an OTLP endpoint and a file destination are configured
- **THEN** the span reaches the collector and is appended to the NDJSON file

### Requirement: Incoming W3C trace context
The framework SHALL adopt a valid W3C `traceparent` header from an incoming request, continuing that trace with its trace id, parent span id, and sampled flag. An absent or invalid header SHALL start a new trace.

#### Scenario: Valid traceparent
- **WHEN** a request arrives with a valid `traceparent` header
- **THEN** the request span uses the incoming trace id, names the incoming span as its parent, and inherits its sampled flag

#### Scenario: Invalid traceparent
- **WHEN** a request arrives with a malformed, zero, or unsupported `traceparent` header
- **THEN** the request span starts a new trace instead of failing the request

#### Scenario: Trace state preserved
- **WHEN** a request arrives with a `tracestate` header
- **THEN** the trace state is kept on the request span and emitted with outgoing context

### Requirement: Outgoing context injection
The framework SHALL expose the active span context so a handler can write `traceparent` and `tracestate` into outgoing request headers.

#### Scenario: Handler propagates context
- **WHEN** a handler injects the current context into a downstream request
- **THEN** the downstream request carries the same trace id and the current span id as its parent

#### Scenario: No active span
- **WHEN** tracing is disabled and a handler injects context
- **THEN** no trace headers are added

### Requirement: Sampling
The tracer SHALL support always-on, always-off, trace-id ratio, and parent-based sampling. A span that is not sampled SHALL keep its identifiers but SHALL NOT be exported.

#### Scenario: Always off
- **WHEN** the always-off sampler is configured and requests complete
- **THEN** no span is exported while span identifiers remain available to the handler

#### Scenario: Trace-id ratio
- **WHEN** the trace-id ratio sampler is configured with a ratio
- **THEN** the sampling decision is derived deterministically from the trace id and the configured ratio

#### Scenario: Parent-based sampling
- **WHEN** a parent-based sampler receives an incoming trace context marked as not sampled
- **THEN** the request span and its children are not exported

### Requirement: OTLP resource attributes
OTLP export SHALL attach resource attributes describing the service and runtime.

#### Scenario: Service identity
- **WHEN** the application has a name from its configuration or from `OTEL_SERVICE_NAME`
- **THEN** the exported resource carries that value as the service name

#### Scenario: Runtime details
- **WHEN** spans are exported
- **THEN** the resource carries the host name, the runtime name and version, and the service version when it is known

### Requirement: OTLP environment configuration
Tracing SHALL read the standard OpenTelemetry environment variables for the endpoint, headers, service name, and sampler, with explicit options taking precedence.

#### Scenario: Endpoint enables tracing
- **WHEN** no tracer is supplied and an OTLP endpoint variable is set
- **THEN** tracing is enabled and exports to that endpoint without requiring the local tracing variables

#### Scenario: Base endpoint path
- **WHEN** the base endpoint variable is set without a signal path
- **THEN** the exporter appends the traces path to it

#### Scenario: Explicit traces endpoint
- **WHEN** the traces endpoint variable is set
- **THEN** the exporter uses that value unchanged

#### Scenario: Headers
- **WHEN** the OTLP headers variable contains key-value pairs
- **THEN** every pair is sent as a request header

#### Scenario: Explicit option precedence
- **WHEN** an explicit tracing option and an environment variable configure the same setting
- **THEN** the explicit option wins
