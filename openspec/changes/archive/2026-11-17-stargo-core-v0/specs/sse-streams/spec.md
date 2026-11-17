# Spec Delta

## Purpose

Defines the Server-Sent Events response used to push HTML and signals to a Datastar client, including event encoding, headers, flushing, and lifecycle rules.

## ADDED Requirements

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

### Requirement: No stream compression
The framework SHALL NOT compress Server-Sent Events responses.

#### Scenario: Compression disabled
- **WHEN** an event stream response is produced
- **THEN** no `Content-Encoding` header is set by the framework
