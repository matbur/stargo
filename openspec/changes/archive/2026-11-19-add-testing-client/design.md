# Design

## Context

See `proposal.md` for motivation. Current state:

- `*stargo.App` implements `http.Handler`, so any standard test server can serve
  it.
- `forms.Read` expects a real `*http.Request` body, and `cookies.Read` reads
  request headers, so a client that builds real requests exercises both.
- `sse.Stream` writes `text/event-stream` frames with `event:` and `data:`
  lines; nothing parses them back.
- `internal/render.HTML` already renders the four page input forms (context
  renderer, writer renderer, string, bytes).
- Tests across the repo hand-roll `httptest` recorders and `strings.Contains`.
- The module has one dependency (`gomponents`); the testing package should not
  add another.

## Goals / Non-Goals

**Goals:**

- Real HTTP semantics in tests: cookie jar, redirects, headers, framing.
- One place to encode forms and multipart bodies and to read SSE patches.
- Assertions that fail the current test with enough context to debug.
- No new module dependency.

**Non-Goals:**

- Virtual time, tracer assertions, or compressed response decoding.
- A DOM or CSS selector engine for HTML assertions.
- A new server implementation; the client uses `net/http/httptest`.

## Decisions

### Package path `testing`, package name `stargotest`

The directory mirrors the Stario `stario.testing` package, but the Go package is
named `stargotest` so that a test file can import the standard `testing` package
and this one without aliasing either. The doc comment shows the canonical
import.

```go
import (
	"testing"

	stargotest "github.com/matbur/stargo/testing"
)
```

The alias is only needed when both names would be ambiguous; `stargotest` is the
declared package name, so the alias documents intent rather than renaming.

### `httptest.Server` rather than a custom transport

`NewClient(tb, handler, opts...)` starts `httptest.NewServer(handler)` and builds
an `http.Client` whose transport is the server's. This gives real request and
response framing, real `Set-Cookie` parsing, real redirect handling, and real
streaming. A custom in-memory `RoundTripper` would avoid the ephemeral loopback
port but would have to reimplement connection state, flushing, and client
disconnects; rejected because fidelity matters more than avoiding a local port.

`tb.Cleanup` closes idle connections and the server, and `Close` does the same
early. Relative paths resolve through `Client.URL`, which joins them to the
server base URL.

### Testing interface instead of `error` returns

The client is constructed with a small local interface, `TB`, holding
`Helper`, `Fatalf`, and `Cleanup`. `*testing.T` and `*testing.B` satisfy it
directly. `testing.TB` is rejected because it has an unexported method, so no
other package can implement it; the testing package's own tests need a fake
that records failures. Every helper that can fail (transport, decoding,
assertions, SSE parsing) reports through `tb.Fatalf`, so call sites stay linear:

```go
response := client.Get("/rooms")
response.AssertStatus(http.StatusOK).AssertContains("Design")
```

`Fatalf` records the failure and stops the test goroutine, so chained assertions
never run on an invalid value. The alternative — returning `error` everywhere
and offering `require`-style wrappers — doubles the API surface for no benefit in
tests.

### Cookie jar and redirect policy are client options

`WithoutCookies()` leaves `Jar` nil. `WithoutRedirects()` and
`WithMaxRedirects(n)` configure `CheckRedirect`: the first returns
`http.ErrUseLastResponse` so the 3xx response is returned without an error; the
second counts hops and returns an error once the limit is exceeded, which the
client reports as a test failure. `WithDefaultHeader` adds a header that a
per-request header can override.

### Request options build the body, `Do` sends it

`Client.Request(method, path, opts...)` returns an `*http.Request`; `Do` sends it.
Request options cover the encodings:

- `WithHeader(name, value)`.
- `WithBody(contentType string, body []byte)`.
- `WithJSON(value any)`.
- `WithForm(url.Values)` — `application/x-www-form-urlencoded`.
- `WithMultipart(Multipart)`.

`Get`, `PostForm`, and `PostMultipart` are convenience wrappers around the same
path, so they cannot drift. Multipart encoding uses `mime/multipart` with a
random boundary and field names sorted for deterministic bodies; files keep
caller order, and an empty content type becomes `application/octet-stream`.

`Multipart` is a value with `Fields url.Values` and `Files []File`, where `File`
carries `Field`, `Name`, `ContentType`, and `Content`. A map of files would lose
repeated fields; a slice keeps them.

### Buffered responses hold the whole body

`Do` reads the body once and closes it, storing status, headers, body, and the
request on `Response`. `Text` converts the bytes to a string. `DecodeJSON` uses
`encoding/json` and reports a failure with the body on a decode error. Reading
the body twice is not supported by design; tests that stream use `Open` instead.

### Assertions are chainable and truncate bodies

`AssertStatus`, `AssertHeader`, `AssertContains`, `AssertNotContains`, and
`AssertCount` return the response, so calls chain. Messages include the request
method and URL, the expected value, the actual value, and for body assertions the
body truncated to 8 KiB with a marker, so a failing test does not dump a whole
page. Counts are non-overlapping `strings.Count` occurrences.

No HTML parser is added. Fragment assertions answer the questions existing tests
ask without pulling in a DOM dependency, and `Render` covers the other half:
it renders a page value with `internal/render` so tests compare the same bytes
that responses and patches send.

### SSE uses a line reader, not `bufio.Scanner`

`Client.Open(request)` sends a streaming request and returns a `Stream` holding
the response, a `bufio.Reader`, and the test interface. Status and headers are
available immediately. `Next` reads one event block: lines up to a blank line,
`field: value` parsing with one optional leading space, `data` lines joined with
`\n`, `event`, `id`, and `retry` in milliseconds, and comment lines collected in
order. CRLF is stripped. A partial block at EOF is returned so a producer that
omits the final blank line still parses; the next call returns false.

`bufio.Scanner` is rejected because element patch payloads can exceed its
default 64 KiB token limit.

### Patch parsing is a pure function on an event

`Event.Patch()` maps the event type to `PatchElements` or `PatchSignals` and
returns `ErrNotPatch` for anything else. Element data lines `key value` fill
mode, selector, namespace, `useViewTransition`, and `viewTransitionSelector`;
`elements` lines are joined with `\n`, so multi-line HTML round-trips. Signal
events join their `signals` lines and decode the JSON object; `onlyIfMissing`
sets a flag. Unknown keys are ignored.

`Stream.NextPatch` loops over `Next`, skips `ErrNotPatch` events such as pings,
and reports parse failures through the test interface with the raw event. The
raw block is kept on `Event` for diagnostics.

### Placement

- `testing/client.go`: `Client`, `NewClient`, options, `URL`, `Request`, `Do`,
  `Get`, `PostForm`, `PostMultipart`, `Open`, `Close`, `Cookie`.
- `testing/response.go`: `Response`, accessors, and assertions.
- `testing/multipart.go`: `Multipart`, `File`, and the encoder.
- `testing/sse.go`: `Stream`, `Event`, `Patch`, and parsers.
- `testing/render.go`: `Render`.
- `testing/doc.go`: package documentation and the trust/limits notes.

Tests live beside the package and use a small in-test `stargo.App` to prove the
client works with `forms`, `cookies`, and `sse` end to end.
