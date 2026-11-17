# Stargo benchmarks

Two suites, one per layer we care about:

- **Rendering** (`internal/render/bench_test.go`, `examples/hello/app/views_bench_test.go`) — the cost of turning the four accepted HTML forms into bytes, plus the example's real views.
- **HTTP and SSE** (`bench_test.go` at the repository root) — the in-process request path and per-event Server-Sent Events encoding.

The shape follows Stario's `benchmarks/html` and `benchmarks/server/inproc`
suites: repeatable local signal for comparing two checkouts on the same machine,
not lab-grade numbers for comparing machines.

## Run

```sh
make bench
```

The target is `go test -run '^$' -bench . -benchmem ./...`. Run one suite or one
case with the standard Go flags:

```sh
go test -run '^$' -bench . -benchmem ./internal/render
go test -run '^$' -bench BenchmarkServeHTTP -benchmem .
go test -run '^$' -bench BenchmarkSSE -benchmem .
go test -run '^$' -bench 'BenchmarkServeHTTP/plaintext' -benchmem .
go test -run '^$' -bench 'BenchmarkHTML/page' -benchmem ./internal/render
```

`ns/op` is the cost of one call, `B/op` the bytes allocated, and `allocs/op` the
allocation count. Compare repeated runs (`-count 5`) on a quiet machine; a
single run moves by a few percent.

## Rendering

`BenchmarkHTML` covers each accepted value form:

| Case | What it measures |
|---|---|
| `string` | Trusted string copied straight to the response buffer. |
| `bytes` | Byte slice passed through without copying. |
| `context_renderer` | A value implementing `Render(context.Context, io.Writer)`. |
| `writer_renderer` | A value implementing `Render(io.Writer)`, the gomponents shape. |
| `page_prebuilt` | A 50-row products table rendered once and reused. |
| `page_built` | The same table built and rendered per call, like Stario's `page_*`. |
| `example_page` | The counter page from `examples/hello/app`. |

`BenchmarkPage` and `BenchmarkCounterRender` in the example module measure the
views as the application actually builds them.

## HTTP

`BenchmarkServeHTTP` feeds requests straight into `App.ServeHTTP`. Requests and
the response writer are reused, and the writer keeps headers, counts body bytes,
and discards them. Nothing touches a socket, so the numbers isolate Stargo plus
`ServeMux`: dispatch, context setup, middleware chain, handler, and response
writing.

| Case | Stario equivalent | Notes |
|---|---|---|
| `plaintext` | `plaintext` | `c.Text("Hello, World!")`. |
| `json` | `json` | Map value through `c.JSON`. |
| `path_param` | `user_42` | `/user/{id}` read with `PathValue`, echoed as JSON. |
| `not_found` | — | Mux 404 path and unmatched-request tracing. |
| `method_not_allowed` | — | Mux 405 path. |
| `page` | `page_identity` | 50-row table built per request, sent with `c.HTML`. |
| `traced_plaintext` | `traced_json` | Span plus NDJSON encoding to `io.Discard`. |
| `json_post` | `validate_post` | Decodes `{"name":"Ada","age":42}` and answers with `valid: true`. |
| `stream` | `sse_patch` | One request emitting four element and four signal patches. |
| `example_page` | — | The real `/` handler from `examples/hello`. |

`ServeMux` rejects unknown methods and paths itself. The 404 and 405 cases still
cover Stargo's post-handler work: status capture, unmatched-request tracing, and
the console line hook.

## SSE

`BenchmarkSSE` measures per-event encoding on one open stream, matching
Stario's per-event numbers:

| Case | What it measures |
|---|---|
| `patch_elements` | Rendering a `div.card` and encoding one element patch. |
| `patch_signals` | Marshalling a signal map and encoding one signal patch. |

The first iteration sends headers; every later iteration measures only the
event write path.

## What is not measured

- **HTTP parsing, TLS, and transport writes.** These live in `net/http`, outside
  Stargo. The in-process suite deliberately skips them, as Stario does.
- **Compression.** Stargo has no SSE compression, so there is no identity/coding
  split.
- **Browser header parsing.** `browser_headers` and the HTTP/2 cases in Stario's
  suite measure its protocol implementation; Go parses headers before a handler
  runs, so an in-process `ServeHTTP` benchmark cannot see that cost.
- **Network throughput and concurrency scaling.** Use a load generator against
  `examples/hello` when that matters.

## Comparison with Stario

These numbers come from one machine (Apple M1 Pro, macOS, Go 1.27, CPython
3.14.2, Stario 5.8.3 from the local checkout) and one afternoon. Stario ran its
own `benchmarks/server/inproc.py` (asyncio, in-process protocol, best of five)
and `benchmarks/html/micro.py`; Stargo ran the benchmarks in this directory.
Times are microseconds per call.

| Case | Stargo | Stario | Stario / Stargo |
|---|---:|---:|---:|
| `plaintext` | 0.37 | 1.80 | 4.9x |
| `json` | 0.80 | 3.24 | 4.0x |
| `/user/42` | 1.04 | 3.77 | 3.6x |
| `validate` | 1.97 | 6.42 | 3.3x |
| traced plaintext | 3.15 | 10.07 | 3.2x |
| 50-row page | 46.4 | 41.1 | 0.89x |
| SSE elements | 0.55 | 2.21 | 4.0x |
| SSE signals | 1.10 | 3.62 | 3.3x |
| render, small static tree | 0.43 | 0.18 | 0.42x |

Two caveats decide how much these ratios mean:

- **Stario's in-process suite parses raw HTTP bytes; Stargo's reuses an
  `*http.Request`.** The small-response ratios therefore include an HTTP parse
  on the Stario side and none on the Stargo side. The page row is the useful
  one: Stargo is about 13% slower on the 50-row page even with that advantage,
  because building and rendering the gomponents tree costs more than Stario's
  optimized tree.
- **A tiny render is slower in gomponents.** `render walk` is 0.18 µs for
  Stario's three-node tree and 0.43 µs through `render.HTML`, which allocates a
  buffer and copies the result. Stargo's advantage appears at the request
  layer, not in the renderer.

These are handler-layer numbers. They say nothing about sockets, `net/http`
parsing, TLS, or throughput under a load generator. Stario's network results
(`stario/benchmarks/RESULTS.md`) come from a different machine and include the
whole stack, so they cannot be divided into or compared with the table above.
Measure both servers over loopback on the same machine when throughput matters.

When adding a case, keep it in the owning package, reuse the request and
response writer, validate the response status once before `b.ResetTimer`, and
call `b.ReportAllocs`.
