## Why

Applications have no way to serve CSS, JavaScript, fonts, or images. Examples
inline CSS, and any real deployment needs cache busting, immutable headers,
precompressed variants, and path safety. Roadmap item 5.3 covers that gap.

## What Changes

- Add an `assets` package with `FS(fs.FS)` and `Dir(path)` handlers.
- Hash every allowed file at startup, expose `URL(name)` for immutable hashed
  paths and `StableURL(name)` for revalidated plain paths, with an optional
  mount prefix.
- Serve precompressed `.br`, `.zst`, and `.gz` siblings when the client accepts
  them, otherwise the identity file.
- Apply cache policy, ETag, conditional requests, and range requests through
  `http.ServeContent`.
- Restrict serving to an explicit content-type allowlist, reject traversal,
  hidden paths, and symlinks, and allow only `GET` and `HEAD`.
- Add `App.HandleHTTP` so a standard `http.Handler` can be mounted without the
  `Ctx` handler type.
- Update README, package docs, and roadmap.

## Capabilities

### New Capabilities

- `static-assets`: hashed and stable static file serving from an `fs.FS` or a
  directory, with cache policy, precompressed variants, conditional and range
  requests, an allowlist, and path safety.

### Modified Capabilities

- `http-app`: adds mounting for standard `http.Handler` values with the same
  request span treatment as `Ctx` handlers.

## Impact

- New `assets` package and tests.
- `app.go` and app tests for `HandleHTTP`.
- `internal/encoding` extraction shared with `sse`.
- `README.md`, `ROADMAP.md`.
- No new module dependencies.
