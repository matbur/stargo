# tracing Specification

## Purpose

Defines request tracing: every request is a span in a tree, and finished spans are exported as newline-delimited JSON for local querying.

## Requirements

### Requirement: Request spans
The framework SHALL open one span per request and close it when the handler finishes, is cancelled, or panics.

#### Scenario: Successful request
- **WHEN** a request completes successfully
- **THEN** the exported span contains route pattern, method, path, status, start time, end time, and duration

#### Scenario: Client disconnect
- **WHEN** a client disconnects during a live stream
- **THEN** the span ends with attribute `request.client_left` and is not marked as failed

#### Scenario: Handler failure
- **WHEN** a handler returns an error or panics
- **THEN** the span is marked as failed, carries the error message, and records an `exception` or `panic` event

### Requirement: Span kinds
Every span SHALL carry a kind of `request`, `stream`, or `startup`.

#### Scenario: Live handler kind
- **WHEN** a handler owns a long-lived SSE response
- **THEN** its span kind is `stream` so query results can separate it from short requests

### Requirement: Nested steps
A span SHALL create child spans, and a child SHALL reference its parent and trace.

#### Scenario: Child span
- **WHEN** a handler opens a step named `db.load` inside a request span
- **THEN** the exported child has the same trace id and the parent's span id

#### Scenario: Step on closed span
- **WHEN** a step is created from a span that already ended
- **THEN** the call returns an error instead of creating an orphan

#### Scenario: Unclosed step
- **WHEN** a step is never ended
- **THEN** it is not exported

### Requirement: Attributes and events
Spans SHALL accept attributes and timestamped events, with events nested inside the span record on export.

#### Scenario: Attribute export
- **WHEN** an attribute is set on a span
- **THEN** it appears under `attributes` in the exported record

#### Scenario: Event export
- **WHEN** an event is recorded
- **THEN** it appears under `events` with its name, timestamp, and optional attributes

### Requirement: Event cap
A span SHALL keep at most 1000 events and SHALL record the number of dropped events as attribute `span.dropped_events`.

#### Scenario: Event overflow
- **WHEN** more than 1000 events are recorded on one span
- **THEN** only the first 1000 are exported and `span.dropped_events` holds the remainder

### Requirement: NDJSON export
Finished spans SHALL be written as one JSON object per line, one line per span.

#### Scenario: Record shape
- **WHEN** a span is exported
- **THEN** the line contains `span_id`, `trace_id`, `name`, `kind`, `start_ns`, `end_ns`, `duration_ns`, and `status`, plus optional `parent_id`, `error`, `attributes`, and `events`

#### Scenario: Parseability
- **WHEN** the output file is read line by line
- **THEN** every complete line parses as a JSON object

### Requirement: Batched non-blocking writer
Export SHALL enqueue spans to a bounded queue and write them in batches from one writer goroutine. A full queue SHALL drop spans and count the drops instead of blocking a handler.

#### Scenario: Batch flush
- **WHEN** spans are enqueued and the batch size or flush interval is reached
- **THEN** the pending lines are written and flushed together

#### Scenario: Queue full
- **WHEN** the queue is full
- **THEN** the new span is dropped, the drop counter increases, and the handler continues

### Requirement: Configuration and default off
Tracing SHALL be off by default and SHALL be enabled through environment configuration. When off, span operations SHALL be no-ops.

#### Scenario: Tracing disabled
- **WHEN** no tracing environment variable is set
- **THEN** no file is written and span calls have no observable effect

#### Scenario: File destination
- **WHEN** tracing is enabled with a file path
- **THEN** spans are appended to that file

#### Scenario: Stdout destination
- **WHEN** the destination is `-`
- **THEN** spans are written to standard output

### Requirement: Shutdown drain
Closing the tracer SHALL stop accepting new spans, drain queued spans within a bounded time, flush, and close the destination.

#### Scenario: Clean shutdown
- **WHEN** the process shuts down normally
- **THEN** queued spans are written before the destination is closed

### Requirement: Rotation opt-in
File rotation SHALL be off by default and configurable by maximum size and file count.

#### Scenario: Rotation enabled
- **WHEN** rotation is configured with a size of 64 MiB
- **THEN** the writer starts a new file after the current file exceeds that size and keeps the configured number of files

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
