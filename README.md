# stargo

A small Go web framework modeled on [Stario](https://github.com/stariodev/stario):

- HTML built in Go with [gomponents](https://maragu.dev/gomponents), no adapter package.
- Live pages over Datastar Server-Sent Events.
- A process-local relay for waking live handlers.
- Human-readable console output in a terminal: startup, requests, shutdown.
- Request tracing as newline-delimited JSON, off by default.
- `net/http` underneath, with a framework-owned server lifecycle.

Go 1.27 or newer. The module has one direct dependency, and only the example
imports it.

```sh
go get github.com/matbur/stargo
```

## Hello

A route handle is declared once and used both for registration and for links:

```go
var HOME = route.Get("/")

func home(c *stargo.Ctx) error {
	return c.HTML(h.Doctype(h.HTML(
		h.Head(h.TitleEl(g.Text("Hello"))),
		h.Body(h.H1(g.Text("Hello, Stargo"))),
	)))
}

func main() {
	app := stargo.New()
	defer app.Close()
	app.Handle(HOME, home)

	if err := app.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
```

`c.HTML` accepts a gomponents node, a value implementing
`Render(context.Context, io.Writer) error`, a value implementing
`Render(io.Writer) error`, a `string`, or a `[]byte`. Strings and bytes are
trusted HTML and are written unchanged; escaping belongs to the renderer.

## The live-page model

A live page uses three routes:

| Route | Purpose |
|---|---|
| `GET /` | Render the complete document once. |
| `GET /live` | Own a long-lived SSE response for one browser connection. |
| `POST /command` | Read intent, change state, publish. Never render. |

Commands change state and publish a value. Live handlers subscribe, re-render,
and stream element patches. They never mutate shared state, so the same view
component serves both the initial page and every patch.

The runnable example in `examples/hello` is a counter:

```sh
make run
# open http://localhost:8080
```

Its routes:

```go
var (
	Home    = route.Get("/")
	Live    = route.Get("/live")
	Command = route.Post("/command")
)
```

The live handler opens a stream, subscribes to the relay, sends the current
view, then patches on each published value:

```go
stream, err := c.Stream()
subscription, err := hub.Subscribe(c.R.Context(), "counter")
defer subscription.Close()

stream.PatchElements(views.Counter(count),
	sse.WithSelector("#counter"), sse.WithMode(sse.ModeOuter))

for {
	_, value, err := subscription.Next(c.R.Context())
	stream.PatchElements(views.Counter(value),
		sse.WithSelector("#counter"), sse.WithMode(sse.ModeOuter))
}
```

SSE rules:

- One event is one buffered write followed by a flush.
- The protocol default `retry: 1000` is omitted from events.
- Selectors and event ids must not contain line breaks; event ids must not
  contain NUL. Violations return an error before anything is written.
- Streams are never compressed.
- A failed write marks the stream disconnected; the request span records
  `request.client_left` and is not marked failed.

Relay rules:

- Subjects are exact dotted strings. Patterns are exact, `prefix.*`, or `*`.
- Publish never blocks. Each subscription keeps at most one value per subject,
  so a slow browser sees the newest value rather than a backlog.
- `relay.MaxSize(n)` bounds pending subjects and drops the oldest;
  `relay.Latest()` is `MaxSize(1)`. Drops are counted in `Dropped()`.
- Delivery is process-local only: no durability, replay, or cross-process
  fanout.

## Route handles and links

`Href` substitutes path values in placeholder order and percent-encodes each
value, so a value cannot add a path segment or query:

```go
var ROOM = route.Get("/rooms/{id}")

ROOM.Href("42")                          // /rooms/42
ROOM.Href("a/b")                         // /rooms/a%2Fb
ROOM.URL("42").Query(q).Fragment("top")  // /rooms/42?...?...#top
```

`Href` panics when the value count is wrong or when a static route receives
values. Those are programmer errors; the request panic recovery turns them into
`500` and a `panic` span event instead of a process exit.

## Datastar attributes and actions

`datastar` builds Datastar attributes and fetch actions from typed inputs.
Import it as `ds`:

```go
import (
	"time"

	ds "github.com/matbur/stargo/datastar"
)

// A click handler with a POST action.
h.Button(ds.On("click", ds.Post("/cart")), g.Text("Add to cart"))

// Modifiers compose into one attribute name.
h.Input(ds.On("input", ds.Post("/typing"), ds.WithDebounce(300*time.Millisecond)))

// A form request with an explicit selector.
h.Form(
	ds.On("submit",
		ds.Post("/rooms", ds.WithForm(), ds.WithSelector("#create-room")),
		ds.WithPrevent(),
	),
)

// Initialize the page and seed signals that do not exist yet.
h.Body(
	ds.Init(ds.Get("/live")),
	ds.Signals(map[string]any{"count": 0, "open": false}, ds.WithIfMissing()),
)

// Bind an input and one attribute reactively.
h.Input(ds.Bind("message"))
h.Button(ds.Attr("disabled", "!$message"), g.Text("Send"))
```

`On` validates the event name and panics on an invalid one, so a typo fails at
startup instead of silently doing nothing. The same applies to attribute keys
and signal names. `New(prefix)` returns an `Attributes` value for a Datastar
bundle built with another prefix; the prefix must match the bundle, and the
same value must be used for initial pages, SSE patches, and raw HTML.

Action paths are quoted as JavaScript string literals. Payload values and
signal filter patterns are trusted JavaScript, not strings; never pass
user-controlled text there. Signal payloads passed to `Signals` are the
opposite: they are encoded as JSON data, never evaluated as JavaScript.

## Lifecycle

`ListenAndServe` owns start and shutdown. On `SIGINT` or `SIGTERM` it:

1. cancels the application root context, which every request context derives
   from, so live streams end immediately;
2. calls `http.Server.Shutdown` with the shutdown timeout, giving handlers time
   to finish;
3. drains and closes the tracer before returning.

Defaults are stream-safe: `ReadHeaderTimeout` 5s, `WriteTimeout` 0 (a write
timeout would kill SSE), `IdleTimeout` 120s, shutdown timeout 10s.

```go
app.ListenAndServe(":8080",
	stargo.ShutdownTimeout(15*time.Second),
	stargo.ReadHeaderTimeout(5*time.Second),
	stargo.Server(func(server *http.Server) {
		server.MaxHeaderBytes = 1 << 20
	}),
)
```

For tests, unix sockets, or a custom listener, use `Serve` with a root context;
it installs no signal handling:

```go
err := app.Serve(ctx, listener)
```

Handlers must watch `c.R.Context()`. `Shutdown` stops accepting connections and
waits for handlers, but it does not cancel their contexts; a handler that
ignores cancellation is cut off when the shutdown timeout expires, and the entry
point returns `context.DeadlineExceeded`.

## Responses and errors

`Ctx` provides `HTML`, `JSON`, `Text`, `NoContent`, `Redirect`, and `Stream`.
A second complete response returns an error. A handler that returns `nil`
without writing anything produces `204`.

Errors carrying a status are sent with that status:

```go
return stargo.NewHTTPError(http.StatusNotFound, "no such room")
```

Any other error becomes `500` with a generic body, while the real message is
kept in the request span. If the response already started, the connection is
aborted instead of appending an error body.

## Forms

`forms.Read` parses an HTML form submission. `GET` and `HEAD` read the query
string; every other method reads the body, which must be
`application/x-www-form-urlencoded` or `multipart/form-data`.

```go
form, err := forms.Read(c.R)
if err != nil {
	switch {
	case errors.Is(err, forms.ErrTooLarge):
		return stargo.NewHTTPError(http.StatusRequestEntityTooLarge, "form too large")
	case errors.Is(err, forms.ErrUnsupportedMediaType):
		return stargo.NewHTTPError(http.StatusUnsupportedMediaType, "expected a form")
	default:
		return stargo.NewHTTPError(http.StatusBadRequest, "malformed form")
	}
}
defer form.Close()

name := form.Get("name")
tags := form.Values("tag")

if photo := form.File("photo"); photo != nil {
	content, err := photo.ReadAll()
	if err != nil {
		return err
	}
	_ = content
}
```

Parsing is bounded: 10 MiB per body, 1000 fields, 1 MiB per field, and 10 files
by default, and `WithMaxBytes`, `WithMaxFields`, `WithMaxFieldBytes`,
`WithMaxFiles`, `WithMaxFileBytes`, and `WithMemoryBytes` change the limits.
Uploads up to 1 MiB stay in memory; larger ones spool to a temporary file that
`Close` removes. Filenames are reduced to their base name, and both values and
filenames are untrusted.

## Cookies

`cookies` validates a cookie before writing it and returns `ErrInvalid` where a
browser would drop or rewrite it. It also enforces the `__Host-` and
`__Secure-` prefix rules.

```go
cookies.Set(c.W, &http.Cookie{Name: "theme", Value: "dark", Path: "/", HttpOnly: true})
theme := cookies.Read(c.R)["theme"]
cookies.Delete(c.W, &http.Cookie{Name: "theme", Path: "/"})
```

`Read` parses every `Cookie` header line, so the last value for a name wins, and
a malformed pair is skipped. `Delete` sends a clearing cookie with the same
path, domain, and flags, which is what browsers match on.

Signed cookies carry a JSON payload the client can read but not change:

```go
signer, err := cookies.NewSigner("__Host-session", []string{secret})
if err != nil {
	return err
}
if err := signer.Set(c.W, map[string]any{"user_id": userID}); err != nil {
	return err
}
value, ok := signer.Get(c.R)
```

The payload is signed, not encrypted: store an id, never a secret. The first
key signs and every key verifies, so a new key can go first and the old one stay
for one max age. The signing key is derived from the configured key and the
cookie name, so one secret can serve several cookies. Defaults are a 14-day max
age, path `/`, `Secure`, `HttpOnly`, and `SameSite=Lax`.

## Console output

When standard output is a terminal, starting the server prints a startup
block, one line per finished request, and a shutdown block:

```
18:20:15 ● server.startup                                                                       29µs
           app.name=hello listen_mode=tcp host=127.0.0.1 port=8080 tls=false listening=true
           graceful_shutdown_timeout=10s
           timeout  request_header=5s keep_alive=2m0s
18:20:40 ● GET /  200                                                                          965µs
18:20:46 ● server.shutdown                                                                     477µs
         │ shutdown  trigger=expected_stop open_connections=2 urgent=false idle_closed=2
         │           force_closed=0 aborted=0 stale_connections=0 cancelled_tasks=0
         └◦ server.shutdown.closed                                                            +328µs
```

| Variable | Meaning |
|---|---|
| `STARGO_CONSOLE` | `auto` (default): only when stdout is a terminal. `1`, `true`, `on`: always. `0`, `off`, `false`: never. |
| `NO_COLOR` | Set to any value to disable ANSI styling. |
| `COLUMNS` | Terminal width; lines use one cell less, capped at 100. |

Serve options override the environment:

```go
app.Serve(ctx, listener, stargo.Console(os.Stderr)) // explicit destination
app.Serve(ctx, listener, stargo.NoConsole())        // disable
```

`stargo.WithName("hello")` sets the startup name; the default is the executable
base name. Console output is independent of tracing: it works with `TRACE` off,
and enabling it does not change the NDJSON export.

## Tracing

Tracing is off by default: no file is created and span calls are no-ops.

| Variable | Meaning |
|---|---|
| `TRACE` | Unset, empty, `0`, `off`, `false`: off. `-`: stdout. `1`, `true`, `on`: use `TRACE_FILE`. Any other value: that file path. |
| `TRACE_FILE` | File used when `TRACE=1`; defaults to `trace.ndjson`. |
| `TRACE_MAX_MB` | Opt into rotation after this many mebibytes. |
| `TRACE_KEEP` | Total files kept by rotation, including the current one. Default 5. |

```sh
TRACE=trace.ndjson go run ./examples/hello
TRACE=- go run ./examples/hello
TRACE=1 TRACE_FILE=/var/log/stargo.ndjson TRACE_MAX_MB=64 TRACE_KEEP=3 go run ./examples/hello
```

Every finished span is one JSON line:

```json
{"span_id":"...","trace_id":"...","name":"GET /live","kind":"stream","start_ns":1,"end_ns":2,"duration_ns":1,"status":"ok","attributes":{"http.status":200,"request.client_left":true}}
```

`kind` is `request`, `stream`, or `startup`. Nested `Step` spans share the
`trace_id` and name their parent in `parent_id`. Lines may appear in completion
order, so group by `trace_id` when reconstructing a tree. Long-lived streams have
huge durations by nature, so exclude `kind = stream` from latency statistics.

```sh
# Slow short requests.
jq -c 'select(.kind != "stream") | {name, ms: (.duration_ns / 1000000)}' trace.ndjson |
	sort -t: -k2 -nr | head

# Failures and their events.
jq -c 'select(.status == "error") | {name, error, events}' trace.ndjson
```

DuckDB reads the same file directly:

```sql
SELECT name,
       count(*) AS calls,
       round(avg(duration_ns) / 1e6, 2) AS avg_ms,
       round(max(duration_ns) / 1e6, 2) AS max_ms
FROM read_ndjson_auto('trace.ndjson')
WHERE kind <> 'stream'
GROUP BY name
ORDER BY avg_ms DESC;
```

```sql
SELECT status, count(*) AS calls
FROM (
  SELECT coalesce(attributes->>'http.status', '?') AS status
  FROM read_ndjson_auto('trace.ndjson')
  WHERE kind = 'request'
)
GROUP BY status
ORDER BY calls DESC;
```

```sql
SELECT path, count(*) AS left
FROM (
  SELECT attributes->>'http.path' AS path
  FROM read_ndjson_auto('trace.ndjson')
  WHERE kind = 'stream'
    AND attributes->>'request.client_left' = 'true'
)
GROUP BY path;
```

The writer is batched and bounded: 512 spans or 125 ms per flush, a bounded
queue, and dropped spans counted (visible through `Tracer.DroppedSpans`). A full
queue never blocks a request. `Close` drains the queue before closing the file.

## Examples

Three runnable examples build up the framework:

| Example | Shows |
|---|---|
| [`examples/hello-world`](examples/hello-world/) | One route, one handler, one page, no JavaScript |
| [`examples/hello`](examples/hello/) | The live-page model with Datastar and a relay |
| [`examples/chat-room`](examples/chat-room/) | Lobby, per-room pages, SQLite, identity, and live streams |

```sh
make run-hello-world   # http://localhost:8080
make run              # the counter from examples/hello
make run-chat-room    # the chat room, with its own module
```

`examples/chat-room` is a separate Go module so SQLite never becomes a
dependency of the framework. Its README covers routes, persistence, and the
pinned Datastar CDN requirement; `make test-examples` runs both example gates.

## Development

```sh
go vet ./...
go test -race ./...
go build ./...
```

`make test`, `make vet`, `make race`, `make build`, and `make run` wrap the same
commands. `make test-examples` runs the hello-world tests and the full
chat-room gate inside its module.

`make bench` runs the rendering microbenchmarks and the in-process HTTP/SSE
benchmarks. See [`benchmarks/README.md`](benchmarks/README.md) for the case list
and how to read the numbers.

## Not included

No static asset pipeline, sessions or authentication, HTTP/2
tuning, SSE compression, cross-process relay, replay, or OpenTelemetry export.
The relay and the tracer are process-local by design.
