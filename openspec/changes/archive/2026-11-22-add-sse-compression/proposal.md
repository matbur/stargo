## Why

Long-lived Datastar streams repeat large HTML and signal fragments. On a busy
page those repeated fragments dominate bandwidth, and every event currently
goes over the wire uncompressed. The framework also gives no stream-oriented
HTTP/2 tuning beyond the generic `stargo.Server` escape hatch.

## What Changes

- Add negotiated gzip compression for Server-Sent Events. Compression is used
  only when the request advertises gzip, and `Content-Encoding`, `Vary`, and
  the existing headers stay correct.
- Flush the gzip writer and the connection after every event so an event never
  waits inside the compressor.
- Skip compression when the response already has a content encoding, when the
  application marks the response `no-transform`, and for `HEAD` requests.
- Add an `HTTP2Streaming` server preset that raises concurrent stream limits
  and sets stream-oriented ping and write-byte timeouts.
- Update the SSE and HTTP application specs, plus the README and roadmap.

## Capabilities

### Modified Capabilities

- `sse-streams`: replaces the "no stream compression" rule with negotiated
  gzip compression, per-event flushing, and compression safety rules.
- `http-app`: adds a stream-oriented HTTP/2 preset to the server options.

## Impact

- `sse/stream.go`, `sse/options.go`, and `sse` tests.
- `ctx.go` to pass the request into the stream for content negotiation.
- `server.go` and server tests.
- `README.md`, `ROADMAP.md`.
