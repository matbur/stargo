# Tasks

## 1. Console renderer

- [x] 1.1 Add the console block model and pure formatter with clock, glyph, name, status, right-aligned duration, and failure marker; verify with table-driven tests for success, failure, and 404 blocks.
- [x] 1.2 Implement duration and value formatting, including microsecond thresholds, quoted values with spaces, and boolean rendering; verify boundary cases at 9.9µs, 10µs, 1ms, and 1s.
- [x] 1.3 Implement attribute layout: fold the span-name prefix, group keys sharing a dotted label under one dim label, and flow pairs to the line width; verify grouped startup and shutdown blocks in tests.
- [x] 1.4 Implement width selection from `COLUMNS` capped at 100, trailing-space trimming, and optional ANSI dim/bold guarded by terminal detection and `NO_COLOR`; verify plain output is unchanged when color is off.

## 2. Activation and configuration

- [x] 2.1 Add `STARGO_CONSOLE` parsing for `auto`, `1`, and `0`, with `auto` enabling output only when standard output is a character device; verify each value with environment-based tests.
- [x] 2.2 Add `Console(io.Writer)` and `NoConsole()` serve options and resolve the destination once per `Serve` call; verify an explicit writer receives output when stdout is not a terminal, and `NoConsole()` suppresses it.

## 3. Startup summary

- [x] 3.1 Add `WithName` application option and default the name to `filepath.Base(os.Args[0])`; verify default and explicit names in startup-block tests.
- [x] 3.2 Write the startup block when the serving goroutine starts, with address, listen mode, TLS flag, graceful shutdown timeout, header timeout, idle timeout, and measured duration; verify a `Serve` test captures a `server.startup` block with the configured values.

## 4. Request lines

- [x] 4.1 Log one line per request in `ServeHTTP` using the captured start time, method, path, recorder status, and duration; verify success, `404`, `405`, and `500` paths.
- [x] 4.2 Log live streams when the handler returns and use the failure marker for `5xx`; verify a streaming request produces a single line with duration at least the stream lifetime.
- [x] 4.3 Serialize request blocks through one writer mutex and issue one write per block; verify concurrent requests produce complete, non-interleaved blocks.

## 5. Shutdown summary

- [x] 5.1 Track open, idle-closed, force-closed, stale, aborted, and in-flight counts through `ConnState` and a request counter; verify transitions with a server test that opens and closes connections.
- [x] 5.2 Write the shutdown block after shutdown completes with `expected_stop` or `runtime_failure`, duration, counters, and a nested `server.shutdown.closed` event; verify a normal stop and a handler-timeout stop.
- [x] 5.3 Write the shutdown summary before returning a shutdown timeout error and preserve the existing deadline error; verify the timeout test checks both the error and the reported force-closed count.

## 6. Documentation and integration

- [x] 6.1 Document console output, `STARGO_CONSOLE`, `Console`, and `NoConsole` in `README.md`; verify the documented commands match the implemented behavior.
- [x] 6.2 Run `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./...`; verify the root gate passes.
- [x] 6.3 Run the `examples/hello` and `examples/hello-world` tests; verify both example gates pass.

## Workflow follow-up

- Archive the change after review and sync the `console-logging` spec into main specs.
