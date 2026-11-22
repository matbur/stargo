# http-app Specification

## Purpose

Defines how requests enter the framework: route registration, the handler contract, response helpers, and how failures become HTTP responses.

## Requirements

### Requirement: Route registration
The application SHALL register routes by method and path pattern, using the standard library pattern syntax where `{name}` denotes a path segment and `{name...}` a trailing remainder.

#### Scenario: Method and path match
- **WHEN** a route is registered for `GET /rooms/{id}` and a request arrives for `GET /rooms/42`
- **THEN** the handler receives a context whose path value for `id` is `42`

#### Scenario: Method mismatch
- **WHEN** only `GET /rooms/{id}` is registered and a `POST` request arrives for that path
- **THEN** the framework responds `405 Method Not Allowed`

#### Scenario: Unknown path
- **WHEN** no registered pattern matches the request path
- **THEN** the framework responds `404 Not Found`

### Requirement: Handler contract
A handler SHALL be a function that takes a request context and returns an error. Returning `nil` SHALL mean the handler is finished and the response is complete or intentionally left empty.

#### Scenario: Handler returns nil
- **WHEN** a handler writes a response and returns `nil`
- **THEN** the framework performs no further writes to that response

#### Scenario: Handler returns no response
- **WHEN** a handler returns `nil` without writing any response
- **THEN** the framework responds `204 No Content`

### Requirement: Error to status mapping
A handler error SHALL become an HTTP error response. Errors carrying an HTTP status SHALL use that status; any other error SHALL become `500 Internal Server Error`.

#### Scenario: Typed HTTP error
- **WHEN** a handler returns an error whose HTTP status is `404`
- **THEN** the client receives `404` and the request span is marked as failed with that message

#### Scenario: Unclassified error
- **WHEN** a handler returns an error with no HTTP status
- **THEN** the client receives `500` and the request span is marked as failed

#### Scenario: Error after response started
- **WHEN** a handler has already started a response and then returns an error
- **THEN** the connection is aborted without framing the response as complete, and the span is marked as failed

### Requirement: Panic recovery
A panic inside a handler SHALL NOT terminate the process. The framework SHALL recover it, mark the request span, and respond with `500` unless the response already started.

#### Scenario: Panic before response
- **WHEN** a handler panics before writing a response
- **THEN** the client receives `500` and the span records a `panic` event

#### Scenario: Panic inside a live stream
- **WHEN** a handler panics after the response started
- **THEN** the connection is aborted and the span is marked as failed

### Requirement: Response helpers
The request context SHALL provide helpers to send HTML, JSON, text, an empty response, a redirect, and a Server-Sent Events stream.

#### Scenario: HTML response
- **WHEN** a handler sends HTML
- **THEN** the response has status `200` and `Content-Type: text/html; charset=utf-8`

#### Scenario: JSON response
- **WHEN** a handler sends a JSON value
- **THEN** the response has status `200` and `Content-Type: application/json`

#### Scenario: Redirect response
- **WHEN** a handler redirects with status `307`
- **THEN** the response has status `307`, an empty body, and a `Location` header

#### Scenario: Second response attempt
- **WHEN** a handler attempts to send a second complete response on the same request
- **THEN** the framework returns an error instead of writing again

### Requirement: Route handles
Routes SHALL be declarable once as immutable values carrying a method, a path template, and a combined pattern. Registering a route handle SHALL dispatch on that pattern and SHALL use it as the request span name.

#### Scenario: Single declaration
- **WHEN** a route handle is registered and a matching request arrives
- **THEN** the handler runs and the request span name equals the handle's pattern

#### Scenario: Methodless handle
- **WHEN** a route handle declares no method
- **THEN** requests with any method match that handle

#### Scenario: Immutable value
- **WHEN** a caller reads the method, path, or pattern of a route handle
- **THEN** those values cannot be changed after creation

### Requirement: URL building from routes
A route handle SHALL build a URL by substituting path values in placeholder order, percent-encoding each value so it cannot alter the path structure. Query parameters and a fragment SHALL be appendable.

#### Scenario: Placeholder substitution
- **WHEN** a handle for `/rooms/{id}` is built with value `42`
- **THEN** the result is `/rooms/42`

#### Scenario: Escaped value
- **WHEN** a value contains `/` or `?`
- **THEN** the character is percent-encoded and does not become a new path segment or query

#### Scenario: Wrong value count
- **WHEN** the number of values does not match the number of placeholders
- **THEN** the call fails instead of returning a partial or malformed URL

#### Scenario: Values on a static route
- **WHEN** values are passed to a handle whose path has no placeholders
- **THEN** the call fails

#### Scenario: Query and fragment
- **WHEN** query parameters and a fragment are supplied
- **THEN** they are appended encoded after the path

### Requirement: Application lifecycle
The framework SHALL provide entry points that start an HTTP server and shut it down cleanly: request contexts SHALL be cancelled before the server stops accepting connections, in-flight handlers SHALL be given a bounded time to finish, and tracing SHALL be drained before the entry point returns.

#### Scenario: Clean shutdown ends live streams
- **WHEN** shutdown begins while a live stream handler is running
- **THEN** the handler's request context is cancelled, the handler returns, and the entry point returns without error

#### Scenario: Shutdown timeout
- **WHEN** a handler ignores context cancellation and exceeds the shutdown timeout
- **THEN** the entry point returns a deadline error after the timeout instead of hanging

#### Scenario: Signals
- **WHEN** the process receives an interrupt or termination signal
- **THEN** the framework performs the same clean shutdown sequence

#### Scenario: Server failure
- **WHEN** the listener fails to bind or the server returns an error
- **THEN** the entry point returns that error without waiting for a signal

#### Scenario: Custom listener
- **WHEN** an application supplies its own listener and a root context
- **THEN** the framework serves on that listener, treats context cancellation as shutdown, and does not install signal handling

#### Scenario: Trace drain ordering
- **WHEN** the entry point returns after a clean shutdown
- **THEN** request spans produced before shutdown are already written

### Requirement: Stream-safe server defaults
The default server configuration SHALL NOT apply a write timeout, SHALL apply a read-header timeout, and SHALL allow those defaults to be overridden through options.

#### Scenario: Write timeout disabled
- **WHEN** a long-lived response writes after the default read-header timeout has elapsed
- **THEN** the write still succeeds because no write timeout applies

#### Scenario: Options override
- **WHEN** an application sets a custom shutdown timeout
- **THEN** that value governs how long shutdown waits for handlers

### Requirement: Request span exposure
The request context SHALL expose the request span so handlers can add attributes and events without knowing where spans are exported.

#### Scenario: Handler adds attributes
- **WHEN** a handler adds an attribute to the request span
- **THEN** the attribute appears in the exported span record for that request

### Requirement: HTTP/2 streaming preset
The framework SHALL provide a server option that configures HTTP/2 for many concurrent long-lived streams, including a higher concurrent stream limit, larger receive buffers, health-check pings, and a bounded write-byte timeout.

#### Scenario: Preset applied
- **WHEN** an application serves with the HTTP/2 streaming preset
- **THEN** the underlying server has non-zero concurrent stream, receive buffer, ping, and write-byte timeout settings

#### Scenario: Individual override
- **WHEN** the generic server option runs after the preset
- **THEN** it can replace individual HTTP/2 fields without losing the rest of the preset

#### Scenario: HTTP/1.1 unaffected
- **WHEN** a client connects without HTTP/2 negotiation
- **THEN** the server still serves normal HTTP/1.1 responses
