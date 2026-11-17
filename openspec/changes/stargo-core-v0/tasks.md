# Tasks

## 1. Module scaffold

- [ ] 1.1 Create `go.mod` with module `github.com/matbur/stargo`, `go 1.27`, and only `maragu.dev/gomponents` as a requirement; verify with `go mod tidy` and `go list -m all` showing a single direct dependency.
- [ ] 1.2 Add package skeleton directories (`route`, `sse`, `relay`, `trace`, `signals`, `internal/views`) with doc comments; verify with `go build ./...` and `go vet ./...`.
- [ ] 1.3 Add a `justfile` or `Makefile` with `test`, `vet`, `run` targets; verify each target runs successfully on the empty skeleton.

## 2. Route handles

- [ ] 2.1 Implement the `route` package with `Get`, `Post`, `Put`, `Delete`, and `Any`, parsing `{name}` placeholders once at construction; verify method, path, placeholder names, and pattern for each constructor in table-driven tests.
- [ ] 2.2 Reject invalid definitions at construction (missing path, path without leading `/`, duplicate method, query or fragment in the path); verify each case panics or returns an error as designed.
- [ ] 2.3 Implement `Href` with positional substitution and `url.PathEscape` per value; verify `/rooms/42`, and that values containing `/`, `?`, and `#` are percent-encoded and cannot change the path structure.
- [ ] 2.4 Implement failure behavior for arity mismatch and for values passed to a static route; verify both cases fail instead of returning a partial URL.
- [ ] 2.5 Implement query and fragment appending with `url.Values`; verify encoding, ordering of pairs, and fragment escaping.
- [ ] 2.6 Verify immutability and comparability: two handles with the same method and path compare equal, and no exported field allows mutation.

## 3. HTTP core

- [ ] 3.1 Implement `App` with `Handle(Route, Handler)`, `HandleFunc(pattern string, Handler)`, `ServeHTTP`, and `Use(http.Handler)`; verify registration by route handle, `{id}` path values, and middleware ordering.
- [ ] 3.2 Implement `Ctx` with `W`, `R`, `Span`, and response state tracking; verify that a second complete response returns an error.
- [ ] 3.3 Implement response helpers `HTML`, `JSON`, `Text`, `NoContent`, `Redirect`; verify status codes and `Content-Type` headers in table-driven `httptest` tests.
- [ ] 3.4 Implement error mapping for typed HTTP errors and generic errors, including the "already started" abort path; verify with tests for `404`, `500`, and post-start error abort behavior.
- [ ] 3.5 Implement panic recovery that marks the span and responds `500` or aborts; verify with a handler that panics before and after the first write.
- [ ] 3.6 Use the route pattern as the request span name and verify it matches `Route.Pattern()` for a parameterized route.
- [ ] 3.7 Verify dispatch behavior: unknown path returns `404`, wrong method returns `405`, and a methodless route accepts every method; add `httptest` cases for all three.

## 4. Server lifecycle

- [ ] 4.1 Implement `Serve(ctx, net.Listener, ...ServeOption)` with the stream-safe defaults, `BaseContext` returning the application root context, and a startup span; verify a request is served and that cancelling the context ends a handler blocked on `ctx.Done()`.
- [ ] 4.2 Implement `ShutdownTimeout`, `ReadHeaderTimeout`, and the raw `*http.Server` option; verify each override reaches the constructed server.
- [ ] 4.3 Implement `ListenAndServe(addr, ...ServeOption)` with signal handling and the fixed sequence cancel, `Shutdown`, tracer close; verify with a test that cancels an open stream and asserts the call returns `nil` before the deadline.
- [ ] 4.4 Implement the shutdown-timeout path: verify a handler that ignores the context causes `context.DeadlineExceeded` instead of an unbounded wait.
- [ ] 4.5 Verify stream-safe defaults: a long-lived response keeps writing after the read-header timeout interval because no write timeout applies.
- [ ] 4.6 Verify trace drain ordering: after a clean shutdown, the destination already contains the request span produced before shutdown.

## 5. Rendering

- [ ] 5.1 Add the two renderer interfaces (with and without context) and make `HTML` and SSE patches accept `any`, resolving it by type switch; verify each accepted form is called once and that an unsupported value returns an error.
- [ ] 5.2 Verify gomponents integration needs no adapter package: pass `gomponents.Node` directly and compare the bytes with `Node.Render` output.
- [ ] 5.3 Verify raw `string` and `[]byte` are written unescaped, and render errors before the first byte produce `500` and mark the span.

## 6. SSE streams

- [ ] 6.1 Implement `sse.New(w)` with first-write headers (`200`, `text/event-stream`, `no-cache`) and flush via `http.ResponseController`; verify header values and that a flush occurred.
- [ ] 6.2 Implement `PatchElements` with selector, mode, namespace, view-transition options, event id, and retry; verify exact bytes for the simple and fully-optioned cases.
- [ ] 6.3 Implement multiline payload splitting on CR, LF, and CRLF; verify one `data:` line per source line and no stray CR inside a field.
- [ ] 6.4 Implement `PatchSignals` with dotted-key expansion and only-if-missing; verify the collapsed JSON payload bytes.
- [ ] 6.5 Implement `ExecuteScript`, `Remove`, `Navigate`, and `Ping`; verify each event's exact bytes, including JSON escaping of the navigation URL.
- [ ] 6.6 Implement field validation and stream lifecycle (`started`, `closed`, write-after-close, failed write marks disconnected); verify errors for newline in selector, NUL in event id, and write after close.
- [ ] 6.7 Verify stream integration through `httptest` with a flushing recorder: two sequential patches arrive as two complete events in order.

## 7. Relay

- [ ] 7.1 Implement `Relay[T]` with `Publish` and `Subscribe`, lock-scoped subscriber snapshots, and non-blocking wake; verify concurrent publish and subscribe with `-race`.
- [ ] 7.2 Implement pattern matching for exact, `prefix.*`, and `*`, including the `ab.*` vs `abc.x` boundary; verify with table-driven tests.
- [ ] 7.3 Implement per-subject coalescing and `Latest()`; verify two publishes to one subject deliver only the newer value and two subjects deliver both.
- [ ] 7.4 Implement `MaxSize` with drop-oldest and a drop counter; verify limit `1` keeps only the newest subject and increments the counter.
- [ ] 7.5 Implement `Subscription.Next(ctx)` waiting on wake or context cancellation, and `Close`; verify a closed subscription receives nothing and a cancelled context returns immediately.

## 8. Signals

- [ ] 8.1 Implement `signals.Read` for the `datastar` query parameter on `GET`/`HEAD`/`DELETE` and JSON body otherwise; verify both transports return the decoded object.
- [ ] 8.2 Implement the size limit with a bounded default and an override option; verify oversized payloads return an error and no partial data.
- [ ] 8.3 Reject malformed JSON and non-object JSON; verify both return errors and an absent payload returns an empty result.

## 9. Tracing

- [ ] 9.1 Implement `Span` with `Attr`, `Event`, `Fail`, `Step`, `Do`, and `End`; verify unexported span handling, step-on-closed-span error, and idempotent end.
- [ ] 9.2 Implement the 1000-event cap with `span.dropped_events`; verify the exported record contains the cap and the dropped count.
- [ ] 9.3 Implement the NDJSON writer: bounded channel, batch by 512 spans or 125 ms, `bufio` writes, drop-plus-counter, and drain on `Close`; verify with a slow destination and a full queue.
- [ ] 9.4 Implement rotation (opt-in) and stdout destination (`-`); verify file rollover at the configured size and that `-` writes to standard output.
- [ ] 9.5 Wire request spans into `Ctx`: route pattern, method, path, status, `kind`, client-disconnect attribute, and error/panic events; verify one exported line per request with the expected fields.
- [ ] 9.6 Verify the disabled path: with no environment configuration, no file is created and the request path still runs without allocation of event buffers.

## 10. Example and integration

- [ ] 10.1 Add `examples/hello` with a page route, a live SSE route, and a command route that publishes to the relay, all declared as route handles and served through `ListenAndServe`; verify the page renders and a published event reaches an open stream in a manual run.
- [ ] 10.2 Add an end-to-end test that opens a live stream against a test listener, publishes, asserts the received patch contains the re-rendered HTML, then cancels the context and asserts shutdown completes.
- [ ] 10.3 Verify links built with `Href` reach the registered routes: a test asserts that every href generated in the example resolves to a registered pattern.

## 11. Documentation and verification

- [ ] 11.1 Write `README.md` covering install, the live-page model, the three routes (`GET /`, `GET /live`, `POST /command`), route handles, lifecycle and shutdown behavior, and the SSE/relay usage rules; verify links and code blocks compile conceptually against the example.
- [ ] 11.2 Document tracing: `TRACE`, `TRACE_FILE`, file rotation, and example DuckDB and `jq` queries excluding `kind = stream`; verify each query runs against a generated sample file.
- [ ] 11.3 Run the full gate: `go vet ./...`, `go test -race ./...`, and `go build ./...` all pass; record the commands in the README.
