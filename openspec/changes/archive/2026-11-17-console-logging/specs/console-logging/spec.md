# Spec Delta

## Purpose

Defines the human-readable console output that makes a running Stargo server observable during development: startup configuration, one line per finished request, and a shutdown summary.

## ADDED Requirements

### Requirement: Console activation
Console output SHALL be enabled automatically when standard output is a terminal, disabled when standard output is redirected, and controllable through the `STARGO_CONSOLE` environment variable and a serve option.

#### Scenario: Terminal output
- **WHEN** the server starts with standard output attached to a terminal and no override is set
- **THEN** startup, request, and shutdown lines are written to standard output

#### Scenario: Redirected output
- **WHEN** standard output is a file or a pipe and no override is set
- **THEN** no console lines are written

#### Scenario: Forced on
- **WHEN** `STARGO_CONSOLE=1` is set
- **THEN** console lines are written even when standard output is not a terminal

#### Scenario: Forced off
- **WHEN** `STARGO_CONSOLE=0` is set
- **THEN** no console lines are written

#### Scenario: Explicit destination
- **WHEN** an application supplies its own console writer through a serve option
- **THEN** console lines are written to that writer regardless of the environment

### Requirement: Startup summary
When the listener is ready, the server SHALL write one startup block containing the application name, listen address, graceful shutdown timeout, request header timeout, idle timeout, and the startup duration. Reported values SHALL describe the configuration actually used.

#### Scenario: Startup line
- **WHEN** the server starts listening on `127.0.0.1:8000`
- **THEN** the first line shows the clock, `●`, `server.startup`, and the measured startup duration, and the block names the address and configured timeouts

#### Scenario: Application name
- **WHEN** the application has no explicit name
- **THEN** the startup block uses the executable base name

#### Scenario: Explicit name
- **WHEN** an application name is configured
- **THEN** that name appears in the startup block

### Requirement: One line per request
The server SHALL write exactly one line per request when the request finishes, containing the request start clock time, method, path, response status, and duration.

#### Scenario: Successful request
- **WHEN** a `GET /` request completes with status `200`
- **THEN** one line shows `● GET /`, `200`, and the duration

#### Scenario: Unmatched path
- **WHEN** a request matches no route and the mux responds `404`
- **THEN** the same line format is used with status `404`

#### Scenario: Stream completion
- **WHEN** a live stream handler finishes after the client disconnects or shutdown cancels it
- **THEN** its request line is written once at that point with the full stream duration

#### Scenario: Failed request
- **WHEN** a request completes with a `5xx` status or a handler panic
- **THEN** the line uses the failure marker instead of `●`

#### Scenario: Concurrent requests
- **WHEN** several requests finish at the same time
- **THEN** each request block is written atomically and lines from different requests never interleave

### Requirement: Shutdown summary
After shutdown finishes, the server SHALL write one shutdown block with the shutdown trigger, connection counters, the shutdown duration, and a nested `server.shutdown.closed` event.

#### Scenario: Expected stop
- **WHEN** shutdown is caused by context cancellation or a termination signal
- **THEN** the trigger is `expected_stop`, the block reports open, idle-closed, force-closed, aborted, stale, and cancelled counts, and the event shows its offset from the shutdown start

#### Scenario: Server failure
- **WHEN** serving fails before any shutdown request
- **THEN** the trigger is `runtime_failure` and the block records the failure

#### Scenario: Shutdown timeout
- **WHEN** handlers exceed the shutdown timeout and the server force-closes connections
- **THEN** the block reports a non-zero force-closed count and the shutdown duration

### Requirement: Console formatting
Console lines SHALL follow the Stario TTY layout: a `HH:MM:SS` clock, one status glyph, a bold name, an optional status code, and a right-aligned duration in one column.

#### Scenario: Duration column
- **WHEN** several request lines are written
- **THEN** their durations align in the same right-hand column

#### Scenario: Duration units
- **WHEN** a duration is below one millisecond
- **THEN** it is shown in microseconds with one decimal below ten microseconds and whole microseconds otherwise

#### Scenario: Attribute layout
- **WHEN** a block has attributes
- **THEN** each attribute is written as `key=value` under the header, folded by the span name prefix, with values containing spaces quoted and keys sharing a dotted label grouped on one row

#### Scenario: Line width
- **WHEN** the terminal width is known
- **THEN** lines are limited to one cell less than the terminal width, capped at 100 cells

#### Scenario: Color
- **WHEN** output is a terminal and `NO_COLOR` is unset
- **THEN** structure is dim and failure status is highlighted, and plain text contains the same content when color is disabled

### Requirement: Tracing independence
Console output SHALL NOT depend on tracing being enabled, and enabling console output SHALL NOT change the NDJSON trace export.

#### Scenario: Tracing off
- **WHEN** no trace destination is configured
- **THEN** console lines are still written and no trace file is created

#### Scenario: Tracing on
- **WHEN** tracing is enabled
- **THEN** spans are exported in the existing NDJSON format while console lines are written separately
