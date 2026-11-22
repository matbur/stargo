# Tasks

## 1. Shared encoding negotiation

- [x] 1.1 Extract the `Accept-Encoding` quality parser into `internal/encoding` with a server-preference order, switch `sse` to it, and keep every existing compression test passing.
- [x] 1.2 Test the shared parser with explicit qualities, refusal, wildcards, aliases, and preference ties.

## 2. Asset index and URLs

- [x] 2.1 Add `assets.FS`, `assets.Dir`, and `assets.Prefix`, walk each source once, skip hidden files, symlinks, and unknown extensions, hash contents, and store an allowlisted index; test both sources, unknown extensions, and directory listings.
- [x] 2.2 Add `URL` and `StableURL` with hashed and plain names, prefix handling, panic on unknown names, and a wrong-hash 404; test hashed and stable requests.

## 3. Serving behaviour

- [x] 3.1 Serve hashed paths as immutable and stable paths as revalidated, set a content-derived ETag, and support `If-None-Match`, ranges, and `HEAD` through `http.ServeContent`; test cache headers, 304, 206, and `HEAD`.
- [x] 3.2 Negotiate existing `.br`, `.zst`, and `.gz` siblings with preference brotli, zstd, gzip, fall back to identity, and set `Vary`; test preference, quality, fallback, and `Content-Encoding`.

## 4. Safety and mounting

- [x] 4.1 Reject traversal, hidden segments, backslashes, NUL, and unsupported methods; test each case and a symlink escape from `Dir`.
- [x] 4.2 Add `App.HandleHTTP` with request span, route attributes, trace adoption, and status recording; test mounting an asset handler and a 404 through it.

## 5. Documentation

- [x] 5.1 Document embedded and directory sources, URL helpers, cache headers, precompressed build output, allowlist, and mounting in `README.md`; update package docs and mark 5.3 done in `ROADMAP.md`.

## 6. Verification

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./assets ./sse .`; confirm all are clean.
- [x] 6.2 Run `openspec validate --changes --strict`; confirm the change passes.
