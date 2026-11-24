# Stargo roadmap

Status: sections 1–4 and 5.1–5.4 are implemented. CSRF, flash messages, a
durable session store, relay replay/transports, and debug/filesystem helpers
remain. The rest of section 5 lists optional work that stays out until a
concrete example needs it.

Reference: `stario/` (Python, local clone, not part of this repository).
Compare against the current Go packages.

## 1. Form parsing (highest value)

Gap: Stargo has no request body parsing; README lists forms as not included.

Scope:

- `application/x-www-form-urlencoded` parsing.
- `multipart/form-data` parsing with streaming parts.
- Limits: field count, part size, total body size, memory threshold.
- `UploadFile` value: field name, filename, content type, reader or bytes.
- `Form` value: ordered fields plus files, with a lookup API.
- Errors for malformed, truncated, or oversized bodies.
- Request helpers that work with `net/http` without a new framework dependency.

Depends on: nothing.

Suggested change: `add-form-parsing`.

## 2. Cookies

Gap: Stargo has no cookie helpers; README lists cookies and sessions as not included.

Scope:

- Set and delete cookies with path, domain, max-age, expires, secure, http-only,
  same-site.
- Signed values and signed cookies with key rotation.
- Prefix validation for `__Host-` and `__Secure-`.
- Header encoding for multiple cookies and parsing of request cookies.
- Session support stays separate; cookies are the foundation.

Depends on: nothing. Useful after forms for login-style flows.

Suggested change: `add-cookie-helpers`.

## 3. Remaining Datastar attributes

Gap: latest change covered single-key state, binding, and reactive attributes only.

Scope:

- Map forms: `Attrs`, `Classes`, `Styles`, `Computeds`.
- Event variants: `OnIntersect`, `OnInterval`, `OnSignalPatch`, including filters.
- `Ignore`, `IgnoreMorph`, `Indicator`, `Ref`, `Persist`, `PreserveAttr`,
  `JSONSignals`.
- Pro attributes (`Animate`, `CustomValidity`, `MatchMedia`) only if needed.

Depends on: nothing. Pattern already established, so this is a small batch.

Suggested change: `add-datastar-extra-attributes`.

## 4. Testing helpers

Gap: tests use `httptest` directly; stario has a testing package.

Scope:

- Test client with cookie jar and redirect control.
- Request encoding for form and multipart bodies.
- Helpers to parse SSE events and element/signal patches.
- Assertion helpers for rendered HTML fragments.

Depends on: forms and cookies, ideally implemented after both.

Suggested change: `add-testing-client`.

## 5. Optional or intentionally excluded

Nothing here blocks the framework. Each item is a feature that only becomes
worth designing once an example or a deployment needs it, so the failure mode
to avoid is building a half-generic subsystem against imagined requirements.
The entries below record what exists today, what is missing, the scope a change
would cover, and the concrete trigger that would justify starting it.

### 5.1 OpenTelemetry export

Status: implemented in `add-opentelemetry-export`.

State before this change: `trace` writes NDJSON to a file, stdout, or any `io.Writer`
through `trace.File`, `trace.Stdout`, and `trace.Destination`. Spans carry UUID
trace, span, and parent IDs, a kind, attributes, and events. The writer is
batched and bounded: 512 spans or 125 ms per flush, a bounded queue, dropped
spans counted, optional rotation and `SyncWrites`. Nothing leaves the process
except through that writer.

Gap: there is no OTLP transport, no collector integration, no remote export, no
W3C `traceparent` propagation, and no sampling. Traces cannot be correlated
across services because a request never reads an incoming trace context, and a
collector such as Tempo, Jaeger, or Grafana cannot receive stargo spans.

Scope a change would cover:

- An exporter seam next to the writer, so a tracer can target a file or OTLP
  without forking `trace`.
- OTLP/HTTP export with endpoint, headers, TLS, timeout, retry, and shutdown
  drain; OTLP/gRPC only if it can be added without a heavy dependency.
- Mapping: `trace` kind to OTel span kind, attributes to typed attribute
  values, span events to OTel events, `Fail` to error status, `Step`/`Do` to
  child spans.
- Propagation middleware that reads `traceparent` and `tracestate`, starts a
  server span, and writes the sampled trace onto outgoing calls.
- Sampling: parent-based plus ratio, with the drop count surfaced like
  `Tracer.DroppedSpans`.
- Env-based configuration (`OTEL_EXPORTER_OTLP_ENDPOINT`,
  `OTEL_SERVICE_NAME`, `OTEL_TRACES_SAMPLER`) matching the existing
  `trace` env conventions, with explicit options taking precedence.
- Resource attributes: service name from `stargo.WithName`, service version,
  host, runtime.

Depends on: the span model staying stable, since the mapping table is the whole
change. No new runtime package needed for file tracing to keep working.

Trigger: traces must reach a collector, or a request has to be followed across
more than one process. Local debugging already works with `trace.File` plus
DuckDB queries in the README.

Suggested change: `add-opentelemetry-export`.

### 5.2 SSE compression and HTTP/2 tuning

Status: implemented in `add-sse-compression` for gzip and the HTTP/2 preset,
and in `add-brotli-compression` for brotli. zstd stays out.

Current state: `sse.Stream` writes each event to the `http.ResponseWriter` and
flushes it. There is no compression layer, so every repeated HTML fragment goes
over the wire uncompressed. The server runs on `net/http` defaults; `Serve`
accepts `stargo.Server(func(*http.Server))`, which is the only escape hatch for
HTTP/2 settings. The README documents the stream as plain HTTP/1.1 or HTTP/2
depending on the listener and TLS.

Gap: long-lived Datastar streams send the same element patches repeatedly, and
uncompressed fragments dominate bandwidth on a busy page. Nothing negotiates
`Accept-Encoding` for `text/event-stream`, nothing decides when compression is
safe for a stream, and nothing tunes HTTP/2 for many concurrent long streams.
Compression of an SSE stream is also easy to get wrong: a buffered encoder must
flush after every event or the client sees nothing.

Scope a change would cover:

- Opt-in stream compression with gzip, zstd, and brotli, negotiated from
  `Accept-Encoding` with q-values and a documented server preference.
- Per-event flush with a bounded rolling window; a dictionary reset strategy
  when the window or the patch pattern changes.
- Safety rules: skip compression below a size threshold, skip when the payload
  is already compressed, keep the `Content-Type` and cache headers intact.
- Interaction with proxies: document `X-Accel-Buffering`, `no-transform`, and
  what happens when an intermediary buffers.
- HTTP/2 tuning presets on `Serve`: keepalive, idle timeout, ping, max
  concurrent streams, initial window sizes sized for long streams, and a way
  to verify settings through `http2.Server`.
- Tests that assert time to first byte, event ordering, no interleaving, and
  that a slow reader does not block the publisher.

Depends on: nothing hard beyond `sse.Stream`. Assets (5.3) would reuse the
negotiation logic for precompressed files.

Trigger: production streaming traffic, a mobile-heavy audience, or a reverse
proxy that buffers streams. Examples on localhost do not need it.

Suggested change: `add-sse-compression` and `add-brotli-compression` (both
done; zstd is the remaining encoding), then `tune-http2-streaming` if the
HTTP/2 part grows beyond a preset helper (the preset shipped with
`add-sse-compression`).

### 5.3 Static asset pipeline

Status: implemented in `add-static-assets`. Dev reload stays out; restart to
rehash.

Current state: handlers return HTML and SSE. There is no static file serving, no
`embed.FS` helper, no hashing, no cache headers, and no precompression. The
examples inline CSS inside gomponents views, which is fine for a demo and wrong
for anything larger.

Gap: an application cannot serve `/_assets/app.css` from disk or from an
embedded bundle, cannot emit immutable cache headers for hashed files, cannot
pick a `.br`, `.zst`, or `.gz` sibling, and has no path-traversal guard or
content-type allowlist.

Scope a change would cover:

- `assets.FS(embed.FS, prefix)` and `assets.Dir(path)` returning a handler.
- Content hashing at startup with a `URL("app.css")` helper that returns the
  versioned path, plus a plain name for stable URLs.
- Cache policy: immutable for hashed names, revalidate with ETag for stable
  names, `no-cache` in dev mode.
- Precompressed variants: serve `.br`, `.zst`, or `.gz` when the client accepts
  them and the file exists; fall back to the original.
- Conditional requests (`If-None-Match`, `If-Modified-Since`) and range
  requests via `http.ServeContent` where possible.
- Path safety: clean joins, containment checks, reject `..`, hidden files, and
  symlinks escaping the root.
- A content-type table with an explicit allowlist instead of sniffing.
- Dev reload: optional re-hash on change so a saved file gets a new URL.

Depends on: nothing, but the encoding negotiation overlaps 5.2 and the path
safety overlaps 5.6.

Trigger: the first example that ships real CSS or JavaScript files, or any
deployment that needs cache busting and immutable caching.

Suggested change: `add-static-assets` (done).

### 5.4 Authentication and sessions

Status: `session` is implemented in `add-sessions` and `auth` in
`add-auth-middleware`. CSRF, flash messages, a durable store example, and a
user store remain.

Current state: `cookies` provides `Set`, `Delete`, `Read`, and `SignedCookie`
with key rotation, prefix validation, and the standard attributes. `session`
adds a cookie-backed id, a store contract, `MemoryStore`, sliding and absolute
expiration, rotation, destruction, and loading middleware. `auth` adds argon2id
password hashing, constant-time verification, session-backed `Login`/`Logout`,
and the `RequireUser` gate with a redirect or a custom unauthorized response.
The chat-room example mints a display identity from query parameters and
Datastar signals; that is not authentication.

Gap: an application still has no CSRF token for mutating forms, no registration
or user store, and no durable session store example. `RequireUser` and argon2id
login already cover the request gate.

Scope a change would cover:

- A `session` package: cookie-backed session ID, `__Host-` prefix, pluggable
  store, memory store plus one durable example (SQLite), sliding expiration,
  absolute expiration, rotation on login, `Destroy`, and a per-session data map.
  (Done in `add-sessions`.)
- Store contract that is small enough for Redis or a database later. (Done.)
- Session loading middleware that attaches the session to the request context
  and saves it after the handler, with `SameSite=Lax` and `Secure` defaults.
  (Done.)
- Auth helpers: password hashing with argon2id, constant-time verification, a
  login handler, a logout handler, and `RequireUser` middleware with a
  configurable unauthorized response. (Done in `add-auth-middleware`.)
- CSRF: double-submit token bound to the session, hidden input helper, and
  validation middleware for unsafe methods, including the Datastar POST paths
  the examples use. (Remaining.)
- Flash messages on top of the session data map, if the examples need them.
  (Remaining.)
- A durable SQLite session store example. (Remaining.)

Depends on: `cookies`, which is already implemented. Sessions are the
foundation; authentication is a separate, thinner change on top.

Trigger: a real account model, an admin area, or any multi-user example where
the chat-room's minted identity is no longer acceptable.

Suggested change: `add-sessions` and `add-auth-middleware` (both done), then
`add-csrf-protection`, a durable session store, and flash messages if an
example needs them.

### 5.5 Replay and cross-process relay

Current state: `relay.Relay[T]` is a process-local pub/sub hub. It supports
exact subjects, trailing prefix wildcards, and `*`. Each subscription keeps one
pending value per subject and a bounded number of subjects, `Publish` never
waits, and `Close`/`Next` follow the context. Nothing is persisted, nothing is
replayed, and another process cannot publish into it. `sse.Options` already has
`WithEventID`, but nothing consumes `Last-Event-ID` on reconnect.

Gap: an SSE client that reconnects after a dropped connection misses every
patch published while it was away, because the relay has no history. Two
instances behind a load balancer cannot see each other's events, so a user on
instance A does not see a message published on instance B.

Scope a change would cover:

- Optional per-subject ring buffer with a cursor, and `SubscribeFrom` so a
  reconnecting stream can replay from the last seen event.
- `Last-Event-ID` handling in the SSE path: read the header, map it to a
  cursor, replay the gap, then continue live, with a documented behaviour when
  the cursor has already been evicted.
- A transport interface for `Publish`/`Subscribe` with the in-memory
  implementation as the default, plus Redis Streams, NATS, or Postgres
  `LISTEN`/`NOTIFY` as optional adapters.
- Delivery semantics per transport: at-most-once for in-memory, at-least-once
  for the durable adapters, and a note that patches must stay idempotent
  because replay can repeat a patch.
- Idempotence guidance: full-fragment patches re-render current state, so a
  repeated patch is safe; incremental patches would need deduplication.
- Ordering guarantees within a subject, and what is not guaranteed across
  subjects.

Depends on: `sse.WithEventID` already exists. Durable replay needs a store
choice, which is why the adapters are separate from the ring buffer.

Trigger: more than one app instance, or a client that must not miss patches
after a reconnect. A single-process example does not need either.

Suggested change: `add-relay-replay`, then `add-relay-transports` for the
external adapters.

### 5.6 Debug and filesystem helpers

Current state: there is no debug inspector and no filesystem helper package.
Stario has `debug.py` with a draggable warning-coloured overlay that shows
runtime information, and `filesystem.py` with asset loading, modification-time
reload, precompressed variants, and path safety. Stargo has none of that; the
examples inline everything and rely on the browser and the trace file.

Gap: local development has no quick view of the route table, the current
request's span, or the live relay subjects. Applications that load templates or
assets from disk have no shared helper for safe joins, reload-on-change, or
precompressed sibling lookup.

Scope a change would cover:

- A `debug` package with an HTML overlay component that works with the
  gomponents views the examples already use: route table, matched route,
  request method and path, trace ID, span timings, and relay subjects.
- Enablement through an explicit option or environment variable, off by
  default, with a build-time story so it cannot ship enabled.
- No new production dependency: the overlay is plain HTML, CSS, and a small
  script.
- A `filesystem` package with `Load(root, name)` doing a clean join plus a
  containment check, `ReloadOnChange` for development, and the shared
  precompressed-file lookup used by 5.3.
- Errors that distinguish "outside the root", "not found", and "not readable".

Depends on: the debug inspector wants read access to the route table and the
current span; the filesystem helpers overlap the path-safety and
precompression rules from 5.3. Developer experience only, so it should not
block any production feature.

Trigger: development ergonomics once the framework has enough moving parts
that the trace file is no longer enough, or the first example that loads files
from disk.

Suggested change: `add-debug-inspector`, then `add-filesystem-helpers`.

### Why these stay out

Sessions, assets, and relay transports each pull in a store or a third-party
adapter. Compression and HTTP/2 tuning only matter under real traffic. OTel
export only matters when traces must leave the process. Building them earlier
would freeze decisions the examples cannot yet validate. Revisit an entry when
its trigger appears, and open one change per entry rather than one large
"production readiness" change.

## Recommended order

1. `add-form-parsing` — done
2. `add-cookie-helpers` — done
3. `add-datastar-extra-attributes` — done
4. `add-testing-client` — done
5. Optional items as needed (5.1–5.4 done)
