# Spec Delta

## Purpose

Defines request tracing: every request is a span in a tree, and finished spans are exported as newline-delimited JSON for local querying.

## ADDED Requirements

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
