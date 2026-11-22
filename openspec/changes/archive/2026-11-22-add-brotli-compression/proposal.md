## Why

SSE compression supports only gzip. Brotli compresses repeated HTML and JSON
patches better, and modern browsers advertise it. Adding it closes the
remaining encoding gap from roadmap item 5.2; zstd stays out.

## What Changes

- Negotiate `br` and `gzip` from `Accept-Encoding`, using q-values and the
  server preference `br` then `gzip`.
- Run both encodings through one compressor seam so each event still flushes
  the compressor and the connection.
- Keep `Content-Encoding`, `Vary`, and the existing safety rules correct for
  either encoding.
- Add the `github.com/andybalholm/brotli` dependency, the second direct
  dependency of the module.
- Update the SSE spec, README, package docs, and roadmap.

## Capabilities

### Modified Capabilities

- `sse-streams`: stream compression now covers brotli and gzip with q-value
  negotiation and a documented server preference.

## Impact

- `sse/compression.go`, `sse/stream.go`, and compression tests.
- `go.mod`, `go.sum`.
- `README.md`, `ROADMAP.md`, `sse/doc.go`.
