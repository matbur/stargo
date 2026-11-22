# Design

## Index at startup

Both constructors walk the source once, skip hidden segments, symlinks, and
files outside the content-type allowlist, read every remaining file, and store
it in memory. Startup indexing gives stable hashes, removes per-request path
resolution, and makes symlink escapes impossible after construction.

`FS` walks an `fs.FS`. `Dir` wraps `os.DirFS(root)` and additionally skips
entries whose type reports a symlink. A symlinked root is resolved by the
operating system when files are read; links inside the tree are never followed.

## Names and URLs

A file's stable name is its slash-separated path inside the source, for example
`css/app.css`. Its hashed name inserts a ten-character SHA-256 prefix before
the extension: `css/app.1a2b3c4d5e.css`. `URL` returns the prefixed hashed path
and panics on an unknown name, because asset names are programmer-authored
constants. `StableURL` returns the prefixed stable path.

The optional `Prefix("/assets")` affects URL helpers and path matching. The
handler still needs to be mounted at that pattern in the application.

## Serving

Hashed paths get `Cache-Control: public, max-age=31536000, immutable`. Stable
paths get `Cache-Control: no-cache` so the browser revalidates with the ETag.
The ETag is the content hash quoted, with an encoding suffix when a variant is
used.

`http.ServeContent` provides `If-None-Match`, `If-Modified-Since`, `HEAD`, and
range handling. Content type comes from the explicit allowlist, never from
sniffing. Unknown extensions are not indexed, so they return 404.

Precompressed siblings (`name.br`, `name.zst`, `name.gz`) are loaded next to
their source. Encoding negotiation reuses the shared quality parser with
preference `br`, `zstd`, `gzip`, and only offers variants that exist.
`Vary: Accept-Encoding` is set whenever a file has variants. The identity file
is the fallback.

## Path safety

The request path is cleaned and split into segments. Absolute paths, `..`,
backslashes, NUL bytes, and hidden segments are rejected. Lookup is map-based,
so no filesystem path is built from the request.

## Mounting

`App.HandleHTTP(pattern, handler)` registers a standard handler on the mux and
wraps it with the same request span, route attributes, incoming trace context,
and status recording as `Ctx` handlers. Assets do not need a fake `Ctx`.

## Non-goals

- No bundling, minification, or dev watcher; restart to rehash.
- No compression at request time; only existing precompressed siblings are
  served. Creating `.br` and `.gz` files is a build step.
- No SPA fallback and no directory listings.
- No cache-busting for stable paths beyond the ETag.

## Risks

- Reading every asset into memory is fine for application bundles but wrong
  for large media libraries; those should use a dedicated file server.
- A stale precompressed sibling is served under the identity hash. Builds must
  regenerate variants with the source; documentation states the requirement.
