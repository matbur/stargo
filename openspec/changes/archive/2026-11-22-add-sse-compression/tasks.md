# Tasks

## 1. Compression negotiation

- [x] 1.1 Add a `WithRequest` stream option that reads `Accept-Encoding`, parses q-values case-insensitively, and selects gzip unless it is refused; test gzip, `q=0`, an absent header, uppercase tokens, and a wildcard.
- [x] 1.2 Create the gzip writer on the first event, set `Content-Encoding: gzip` and `Vary: Accept-Encoding`, and leave the existing content type, cache, and buffering headers intact; test the headers and a decoded body.
- [x] 1.3 Pass the request from `Ctx.Stream` and document the direct `sse.New` opt-in; test an application stream with a gzip request.

## 2. Flush and lifecycle

- [x] 2.1 Flush the gzip writer and the connection after every event, propagate write errors into the disconnected state, and close the gzip trailer only for a started stream; test per-event flushes, ordering, and a closed stream body.

## 3. Safety

- [x] 3.1 Skip compression for an existing `Content-Encoding`, `Cache-Control: no-transform`, and `HEAD` requests; test each case.

## 4. HTTP/2 preset

- [x] 4.1 Add an `HTTP2Streaming` serve option with stream-oriented concurrency, receive buffer, ping, and write-byte timeout settings, assigned before the generic server configure callback; test the preset values and that the generic option can override them.

## 5. Documentation

- [x] 5.1 Document gzip negotiation, proxy and `no-transform` behaviour, and the HTTP/2 preset in `README.md`; mark 5.2 done in `ROADMAP.md`.

## 6. Verification

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./sse .`; confirm all are clean.
- [x] 6.2 Run `openspec validate --changes --strict`; confirm the change passes.
