# Tasks

## 1. Negotiation and compressor seam

- [x] 1.1 Parse `Accept-Encoding` into qualities, add brotli to the supported set with preference `br` then `gzip`, and keep `x-gzip` and wildcard handling; test explicit encodings, refusal, quality ordering, ties, and wildcards.
- [x] 1.2 Replace the concrete gzip writer with a compressor interface used by both encodings, set the selected `Content-Encoding`, and preserve per-event flush, close, and safety behaviour; test brotli and gzip bodies, headers, and flush counts.

## 2. Documentation

- [x] 2.1 Update `README.md`, `sse/doc.go`, and `ROADMAP.md` for brotli, the preference order, and the second direct dependency; verify the documented names match the implementation.

## 3. Verification

- [x] 3.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./sse .`; confirm all are clean.
- [x] 3.2 Run `openspec validate --changes --strict`; confirm the change passes.
