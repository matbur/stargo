# Design

## Context

The core framework, the counter example, and the end-to-end test harness already exist. See `proposal.md` for motivation and the two spec deltas for required behavior. The Python chat-room is available as reference at `stario/examples/chat-room` and is git-ignored.

Constraints that shape this port:

- The core module must keep exactly one direct dependency, so example-only dependencies cannot live in the root `go.mod`.
- The Python app relies on features the Go core does not have: static asset attachments, a multi-pattern subscription, `c.alive`, and a markup helper layer. The port must express the same behavior with the existing API.
- Go rejects import cycles, while the Python lobby and room features import each other's URL modules.

## Goals / Non-Goals

**Goals:**

- Two runnable examples that mirror the reference request flow and can be read as a progression: one page, then a multi-feature app.
- A chat-room layout that a reader can copy: configuration, database, shared identity and shell, routes, and one package per feature.
- Tests at the levels the examples are meant to teach: SQL queries, page responses, and real SSE streams.

**Non-Goals:**

- Byte-for-byte HTML or CSS parity with the Python markup.
- Replacing the vendored Datastar bundle with an offline copy.
- Reintroducing a static asset pipeline or a test client into the core.
- Multi-process chat, authentication, moderation, or durable presence.

## Decisions

### D1: Separate module for the chat room

`examples/chat-room` gets its own `go.mod` with a `replace github.com/matbur/stargo => ../..` and `modernc.org/sqlite` as its only extra requirement. `examples/hello-world` stays in the root module.

Alternatives: put both in the root module (rejected: SQLite becomes a dependency of every consumer and contradicts the documented single-dependency surface); vendor a fake in-memory store (rejected: the example exists to show SQLite).

Consequence: the root gate does not compile the chat room. The `Makefile` gets explicit example targets, and the example README documents its own gate.

### D2: `database/sql` with a pure-Go SQLite driver

Use `modernc.org/sqlite` through `database/sql`, with plain query functions that take `*sql.DB`. The schema is applied with one idempotent `Exec` at startup.

The default DSN is `:memory:` and the pool is limited to one connection, because an in-memory SQLite database is per connection. A file DSN raises no such constraint.

Alternatives: a hand-rolled connection wrapper with a mutex (rejected: `sql.DB` is already concurrency-safe); `mattn/go-sqlite3` (rejected: cgo is a poor default for an example); an in-memory map store (rejected: loses the persistence lesson).

Consequence: the in-memory default serializes queries, which is fine for a demo and stated in the README.

### D3: Routes as a leaf package

All route handles live in `app/routes`, which imports only `route`. Lobby and room packages both import it, so neither imports the other just for links.

Alternatives: keep per-feature URL files (rejected: lobby needs room routes and room needs the lobby route, which is an import cycle in Go); build links from strings (rejected: defeats the route-handle model).

Subjects stay with their feature: `lobby` owns `lobby.changed`, and `room` owns helpers for room-scoped subjects, because only the room feature publishes them.

### D4: One relay subscription per live handler

`relay.Subscribe` accepts one pattern, so each room stream subscribes to one room prefix. The lobby stream subscribes to `*` and filters subjects in the loop: anything under `lobby.` plus any `.presence` or `.deleted` suffix triggers a lobby patch.

Subscriptions keep the default bounded size instead of `Latest()`. Per-subject coalescing already collapses repeated events, while a size of one would drop unrelated subjects such as a room creation and a presence change arriving together.

Alternatives: add multi-pattern support to the core (rejected: no second consumer yet); one subscription per pattern in the lobby (workable, but two subscriptions need two waits or a merged goroutine, and filtering keeps the example simpler).

Consequence: the lobby receives room message and typing events it ignores. They coalesce per subject and cost one map update each.

### D5: Presence cleanup through deferred work

The room stream subscribes first, then inserts presence and publishes the presence event, then sends the first patch. A deferred cleanup removes presence and publishes again after the loop, whether it ended by context cancellation, relay close, or room deletion.

Alternatives: rely on a separate disconnect notification (rejected: no such channel in the core); clean up in a goroutine watching the request context (rejected: duplicates the loop's exit path and races with the stream).

Consequence: if the process crashes, presence rows remain until the room is deleted or the process restarts. The README states that presence is process-local and best-effort.

### D6: Commands store first, then answer

The Python commands write `204` before touching the database. Go handlers return errors, and writing before a failure would turn a recoverable error into a connection abort. The port stores state first and then writes `204`, which is indistinguishable to the client.

Routes that cannot proceed (missing room, missing identity, user not present) answer with an SSE navigation, matching the reference. Room creation answers with an SSE navigation after the insert.

### D7: gomponents views with raw Datastar attributes

Views are plain functions returning `g.Node`. Datastar attributes are literal attributes such as `data-signals__ifmissing`, `data-init`, `data-on:submit__prevent`, `data-bind`, and `data-attr`. Conditional classes are computed as strings; conditional children become `nil` in the child list.

The stylesheet lives next to the shared shell and is embedded with `//go:embed`, then rendered in a `style` element. Datastar is loaded from the pinned `v1.0.4` bundle URL used by the reference.

Alternatives: a Datastar helper package (rejected by the core design); a static file route (rejected: the core has no raw-handler registration, and a middleware mount would make asset requests appear as unmatched spans).

Consequence: pages need no stylesheet request, and the only network dependency is the Datastar bundle.

### D8: First paint mints, live reads signals

The shared identity package mints an eight-character id, an adjective-animal name, and a color. Page handlers prefer query parameters when all three are present and otherwise mint. Live and command handlers read the same three values from Datastar signals and reject incomplete identities.

The page embeds the identity with `data-signals__ifmissing` and runs a small script that restores `sessionStorage` values over it, so each tab keeps one identity across reloads.

Alternatives: a cookie session (rejected: the reference deliberately avoids sessions); server-minted identity on the live request (rejected: every reconnect would create a new user and leave stale presence).

### D9: Tests against real streams

Page and command tests use `httptest.NewRecorder`. Live tests start the app on a real listener, open the stream with a cancellable request, scan the event lines, and cancel to assert cleanup. Database tests run against a fresh in-memory database.

Alternatives: a custom test client mirroring the Python one (rejected: `net/http` plus a scanner covers the cases); golden HTML files (rejected: noisy and brittle for a styled example).

Consequence: live tests assert the presence of patch payloads, not exact whole-page bytes.

## Risks / Trade-offs

- **In-memory SQLite is per connection** → set `MaxOpenConns(1)` and document it; file databases are unaffected.
- **Lobby filters a global subscription** → message and typing events reach the lobby handler; coalescing bounds the cost, and the filter keeps behavior correct.
- **Presence survives a crash** → documented as best-effort; restarting clears in-memory state, and file databases can contain stale rows until the room is deleted.
- **Separate module can drift from core** → the `replace` directive builds against the working tree, and the `Makefile` example target runs the example gate in CI-like fashion.
- **Datastar bundle comes from a CDN** → pinned version and an explicit README note; the rest of the example works offline.
- **Inline CSS duplicates styles per document** → one document per page load, no per-patch cost, and no extra route.
- **User text in views** → gomponents escapes text nodes; raw output is limited to the embedded stylesheet and the static send icon.
- **Signal values are untrusted JSON** → every command reads named fields, coerces strings, and applies length or presence checks before storage.

## Migration Plan

Additive only. Create the two example directories, their tests, and documentation; update the root README and Makefile. Rollback deletes the new directories and reverts the two root files. No data migration and no core changes.
