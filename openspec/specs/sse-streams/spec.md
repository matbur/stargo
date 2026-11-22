# sse-streams Specification

## Purpose

Defines the Server-Sent Events response used to push HTML and signals to a Datastar client, including event encoding, headers, flushing, and lifecycle rules.

## Requirements

### Requirement: Stream headers
The first write on a stream SHALL send `200`, `Content-Type: text/event-stream`, and `Cache-Control: no-cache`.

#### Scenario: First event sets headers
- **WHEN** the first patch is written and no headers were sent yet
- **THEN** the response status is `200` with `Content-Type: text/event-stream`

#### Scenario: Non-stream response already started
- **WHEN** a patch is attempted after the response started without an event-stream content type
- **THEN** the call returns an error and writes nothing

### Requirement: One event, one write, one flush
Each event SHALL be encoded into a single byte buffer, written once, and flushed to the client before the call returns.

#### Scenario: Event delivery
- **WHEN** a patch is written on an idle connection
- **THEN** the client receives the complete event without waiting for the next write

### Requirement: Element patching
The stream SHALL send element patches with an optional mode, selector, namespace, view-transition options, event id, and retry interval.

#### Scenario: Whole element patch
- **WHEN** a patch is sent without selector or mode
- **THEN** the event contains `event: datastar-patch-elements` and one `data: elements` line

#### Scenario: Selected mode
- **WHEN** a patch is sent with selector `#row-3` and mode `outer`
- **THEN** the event includes `data: selector #row-3` and `data: mode outer`

#### Scenario: Multiline HTML
- **WHEN** the rendered HTML contains line breaks
- **THEN** every line is sent as a separate `data: elements` line within one event

### Requirement: Default retry omitted
The stream SHALL omit the `retry` field when its value equals the protocol default of 1000 milliseconds.

#### Scenario: Default retry
- **WHEN** a patch specifies retry `1000`
- **THEN** the event contains no `retry` line

#### Scenario: Custom retry
- **WHEN** a patch specifies retry `2500`
- **THEN** the event contains `retry: 2500`

### Requirement: Signal patching
The stream SHALL send signal patches as compact JSON, expanding dotted keys into nested objects.

#### Scenario: Nested key
- **WHEN** a patch sends `{"ui.open": true}`
- **THEN** the event payload is `{"ui":{"open":true}}`

#### Scenario: Only if missing
- **WHEN** a signal patch requests only-if-missing
- **THEN** the event contains `data: onlyIfMissing true`

### Requirement: Script, removal, and navigation
The stream SHALL provide helpers to execute developer-authored JavaScript, remove nodes by selector, and navigate the browser.

#### Scenario: Execute script
- **WHEN** a script helper is called with auto-remove enabled
- **THEN** a `<script data-effect="el.remove();">` element is appended to `body`

#### Scenario: Remove nodes
- **WHEN** a removal helper is called with a selector
- **THEN** the event contains `data: mode remove` and that selector

#### Scenario: Navigate
- **WHEN** a navigation helper is called with a URL
- **THEN** the event patches `body` with a script that assigns the JSON-escaped URL to `window.location`

### Requirement: Heartbeat
The stream SHALL support a comment-only event that keeps intermediaries from closing an idle connection.

#### Scenario: Ping
- **WHEN** a ping is written
- **THEN** the client receives a line starting with `:` and no event payload

### Requirement: Field value safety
Field values carrying selectors, event ids, or options SHALL NOT contain line breaks, and an event id SHALL NOT contain a NUL byte.

#### Scenario: Line break in selector
- **WHEN** a selector contains a newline
- **THEN** the call returns an error and nothing is written

### Requirement: Stream lifecycle
After the stream is closed, further writes SHALL fail. A failed write SHALL mark the stream as disconnected.

#### Scenario: Write after close
- **WHEN** a caller writes a patch after closing the stream
- **THEN** the call returns an error

#### Scenario: Client disconnect
- **WHEN** a write fails because the client disconnected
- **THEN** the stream reports the disconnect and stops accepting writes

### Requirement: Stream compression
The stream SHALL compress events with brotli or gzip when the request advertises an encoding through `Accept-Encoding`, honoring q-values, excluding an explicitly refused encoding, and preferring brotli when qualities tie.

#### Scenario: Gzip accepted
- **WHEN** a stream is created from a request with `Accept-Encoding: gzip`
- **THEN** events are gzip-compressed and the response carries `Content-Encoding: gzip`

#### Scenario: Brotli accepted
- **WHEN** the request advertises `br`
- **THEN** events are brotli-compressed and the response carries `Content-Encoding: br`

#### Scenario: Server preference
- **WHEN** the request advertises `gzip` and `br` with equal quality
- **THEN** the stream selects brotli

#### Scenario: Quality wins
- **WHEN** the request advertises `br;q=0.2, gzip;q=0.9`
- **THEN** the stream selects gzip

#### Scenario: Gzip refused
- **WHEN** the request contains `gzip;q=0`
- **THEN** events are written without compression and no `Content-Encoding` header is set

#### Scenario: Encoding refused
- **WHEN** the request contains `gzip;q=0` and no other accepted encoding
- **THEN** events are written without compression and no `Content-Encoding` header is set

#### Scenario: No acceptable encoding
- **WHEN** the request has no `Accept-Encoding` header
- **THEN** the stream stays uncompressed and keeps the existing headers

#### Scenario: Cache separation
- **WHEN** the stream selects a compression encoding
- **THEN** the response carries `Vary: Accept-Encoding`

### Requirement: Compressed event delivery
When compression is active, each event SHALL be written through the selected compressor, the compressor SHALL be flushed, and the connection SHALL be flushed before the call returns.

#### Scenario: Per-event flush
- **WHEN** an event is written on a compressed stream
- **THEN** the client can decode it without waiting for the next event

#### Scenario: Decodable stream
- **WHEN** a compressed stream is closed
- **THEN** its body is a valid stream in the selected encoding containing every written event in order

### Requirement: Compression safety
The stream SHALL NOT add compression when the response already has a `Content-Encoding`, when `Cache-Control` contains `no-transform`, or when the request method is `HEAD`, and SHALL keep the event-stream content type and cache headers unchanged.

#### Scenario: Existing encoding
- **WHEN** the response already carries a `Content-Encoding` header
- **THEN** the stream writes identity bytes and does not replace that header

#### Scenario: No transform
- **WHEN** the application sets `Cache-Control: no-transform` before the first event
- **THEN** the stream stays uncompressed

#### Scenario: Head request
- **WHEN** the request method is `HEAD`
- **THEN** the stream does not select compression

#### Scenario: Headers preserved
- **WHEN** a compressed stream sends its first event
- **THEN** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and `X-Accel-Buffering: no` are still present
