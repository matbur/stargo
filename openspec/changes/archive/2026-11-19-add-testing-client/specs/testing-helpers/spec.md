# Spec Delta

## Purpose

Defines test helpers for Stargo applications: a loopback HTTP test client with
cookie and redirect control, form and multipart request encoding, response
assertions, rendered-HTML helpers, and Datastar SSE event and patch parsing.

## ADDED Requirements

### Requirement: Test server lifecycle
A test client SHALL serve a supplied `http.Handler` on a loopback test server and SHALL release the server and idle connections when the test finishes.

#### Scenario: Handler under test
- **WHEN** a test creates a client for an application handler
- **THEN** requests reach that handler with the production routing and response path

#### Scenario: Relative paths
- **WHEN** a caller passes a path that does not start with a scheme or host
- **THEN** the client resolves it against the test server's base URL

#### Scenario: Cleanup
- **WHEN** the test ends or the client is closed explicitly
- **THEN** the server stops accepting connections and open idle connections are closed

### Requirement: Cookie jar
A test client SHALL keep a cookie jar by default, storing response cookies and sending matching cookies on later requests.

#### Scenario: Cookie round trip
- **WHEN** one response sets a cookie and a later request targets the same server
- **THEN** the later request carries that cookie

#### Scenario: Disabled jar
- **WHEN** a caller disables cookies
- **THEN** response cookies are not stored and later requests carry no stored cookies

#### Scenario: Cookie lookup
- **WHEN** a caller reads a stored cookie by name
- **THEN** the client reports its value and whether it exists

### Requirement: Redirect policy
A test client SHALL follow redirects by default and SHALL let a caller stop following them or cap how many are followed.

#### Scenario: Follow by default
- **WHEN** a handler redirects to another path
- **THEN** the client returns the final response and its status

#### Scenario: Stop following
- **WHEN** a caller disables redirect following
- **THEN** the client returns the redirect response with its status and location unchanged

#### Scenario: Redirect limit
- **WHEN** a redirect chain exceeds the configured limit
- **THEN** the client reports a test failure instead of returning a partial chain

### Requirement: Default request headers
A test client SHALL apply caller-supplied default headers to every request it sends.

#### Scenario: Header on every request
- **WHEN** a caller adds a default header and sends two requests
- **THEN** both requests carry that header

#### Scenario: Per-request override
- **WHEN** a request sets a header with the same name
- **THEN** the request value is sent instead of the default

### Requirement: Buffered requests
A test client SHALL return a buffered response with status, headers, and the complete body after the handler finishes.

#### Scenario: Simple request
- **WHEN** a caller sends a request to a handler that returns HTML
- **THEN** the response exposes the status, headers, and complete body

#### Scenario: Request body
- **WHEN** a caller sends a request with a body and content type
- **THEN** the handler receives the same bytes and media type

#### Scenario: Transport failure
- **WHEN** the request cannot be sent
- **THEN** the client reports a test failure naming the request target

### Requirement: URL-encoded form requests
A request helper SHALL encode form values as `application/x-www-form-urlencoded` with a single content type.

#### Scenario: Several values
- **WHEN** a caller sends a form with two values for one name
- **THEN** both values arrive in order and the content type is `application/x-www-form-urlencoded`

### Requirement: Multipart form requests
A request helper SHALL encode multipart fields and files with a generated boundary, preserving file names, content types, and repeated fields.

#### Scenario: Text fields and one file
- **WHEN** a caller sends a multipart form with fields and one file
- **THEN** the handler can read every field and the file content, name, and content type

#### Scenario: Default file content type
- **WHEN** a caller sends a file without a content type
- **THEN** the part uses `application/octet-stream`

#### Scenario: Repeated files
- **WHEN** a caller sends two files for the same field
- **THEN** the handler receives both in order

#### Scenario: Boundary
- **WHEN** a multipart request is sent
- **THEN** the content type names the same boundary used in the body

### Requirement: Buffered response access
A buffered response SHALL expose its body as text and SHALL decode a JSON body into a caller-provided value.

#### Scenario: Text body
- **WHEN** a response body is `hello`
- **THEN** reading it as text returns `hello`

#### Scenario: JSON body
- **WHEN** a response body is a JSON object and the caller decodes it
- **THEN** the provided value holds the decoded fields

#### Scenario: Invalid JSON
- **WHEN** the caller decodes a body that is not JSON
- **THEN** the client reports a test failure with the response body

### Requirement: Status and header assertions
A buffered response SHALL provide assertions for its status code and headers that report a failure with the request target, the expected value, and the actual value.

#### Scenario: Matching status
- **WHEN** a caller asserts the status of a matching response
- **THEN** the assertion passes and the response remains usable

#### Scenario: Wrong status
- **WHEN** a caller asserts a status that differs from the response
- **THEN** the test fails and the message names both values

#### Scenario: Missing header
- **WHEN** a caller asserts a header the response does not carry
- **THEN** the test fails and the message names the header

### Requirement: Fragment assertions
A response SHALL provide assertions for whether its body contains or omits fragments and for how many times a fragment occurs.

#### Scenario: Contained fragments
- **WHEN** every asserted fragment occurs in the body
- **THEN** the assertion passes

#### Scenario: Missing fragment
- **WHEN** an asserted fragment does not occur
- **THEN** the test fails and the message shows the missing fragment and the body

#### Scenario: Occurrence count
- **WHEN** a caller asserts a fragment count that differs from the body
- **THEN** the test fails and the message names the expected and actual counts

#### Scenario: Forbidden fragment
- **WHEN** a caller asserts that a fragment is absent and it occurs
- **THEN** the test fails and the message shows the fragment

### Requirement: Rendered HTML helper
A helper SHALL render a page value with the same renderer that responses and patches use.

#### Scenario: Writer renderer
- **WHEN** a caller renders a value implementing `Render(io.Writer) error`
- **THEN** the helper returns the rendered HTML

#### Scenario: String and bytes
- **WHEN** a caller renders a string or a byte slice
- **THEN** the helper returns it unchanged

### Requirement: SSE stream opening
A test client SHALL open a streaming request whose status and headers are available before the first event is read, and SHALL close the body when the caller is done.

#### Scenario: Headers before events
- **WHEN** an SSE handler sends headers and waits before its first event
- **THEN** the caller can assert the status and headers without waiting for an event

#### Scenario: Close
- **WHEN** the caller closes the stream
- **THEN** the response body is closed and later reads stop

### Requirement: SSE event parsing
A stream SHALL parse events with their type, id, retry, data lines, and comments, accepting both CRLF and LF line endings.

#### Scenario: Typed event
- **WHEN** a stream carries an event with a type and two data lines
- **THEN** the parsed event has that type and data lines joined with a newline

#### Scenario: Retry and id
- **WHEN** an event carries `id` and `retry` fields
- **THEN** the parsed event reports the id and the retry duration

#### Scenario: Comments
- **WHEN** a stream carries a comment-only block
- **THEN** the parsed event reports the comment and no type or data

#### Scenario: Event order
- **WHEN** several events arrive in order
- **THEN** reading them returns the same order

#### Scenario: End of stream
- **WHEN** the stream closes after the last event
- **THEN** the next read reports that no event is available

### Requirement: Datastar patch parsing
A stream SHALL parse Datastar element and signal patch events into typed values and SHALL let a caller skip events that are not patches.

#### Scenario: Element patch
- **WHEN** an event patches elements with a mode, selector, and HTML payload
- **THEN** the parsed patch reports the element kind, mode, selector, and payload

#### Scenario: Element patch view transition
- **WHEN** an event requests a view transition and a transition selector
- **THEN** the parsed patch reports both values

#### Scenario: Signal patch
- **WHEN** an event patches signals with JSON data
- **THEN** the parsed patch reports the decoded signal values

#### Scenario: Only if missing
- **WHEN** a signal patch sets the only-if-missing flag
- **THEN** the parsed patch reports the flag

#### Scenario: Skipping other events
- **WHEN** a caller reads patches and a ping event arrives first
- **THEN** the ping is skipped and the next patch is returned

#### Scenario: Invalid patch payload
- **WHEN** a patch event carries an invalid signal payload
- **THEN** the client reports a test failure naming the raw event

### Requirement: Failure reporting
Helper failures SHALL be reported through the test interface passed to the client, not by panicking or writing to standard output.

#### Scenario: Assertion failure
- **WHEN** any assertion fails
- **THEN** the test is marked failed with a message that names the request and the response

#### Scenario: Parse failure
- **WHEN** an SSE event or patch cannot be parsed
- **THEN** the test is marked failed with the raw event
