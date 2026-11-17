# Proposal

## Why

The core change proved that Stario's model works on Go, but one counter is not enough to show it. The Python repository ships three reference apps, and two of them exercise patterns the counter does not: a plain page with no Datastar, and a multi-feature application with SQLite, per-room relay subjects, presence, and command routes. Porting them gives readers runnable comparisons and gives the framework a larger integration test.

## What Changes

- Add `examples/hello-world`, a single-file Go app with one route, one handler, and one static HTML page. It loads no Datastar and no JavaScript.
- Add `examples/chat-room`, a separate Go module that ports the Python chat-room example:
  - lobby with room list and create/delete dialog;
  - rooms with message history, presence, typing indicator, and online counts;
  - SQLite persistence through `database/sql` and `modernc.org/sqlite`;
  - Datastar SSE live handlers for lobby and room, driven by per-room relay subjects;
  - per-tab visitor identity restored from `sessionStorage`;
  - embedded CSS and the pinned Datastar module URL.
- Keep the chat-room example in its own Go module so the core module keeps `maragu.dev/gomponents` as its only direct dependency.
- Add tests for both examples: page rendering, route links, room creation and deletion, unknown rooms, live stream patches, presence cleanup, and SQLite queries.
- Update the root `README.md` and `Makefile` with example commands. The core gate stays unchanged; the example module has its own gate target.

## Capabilities

### New Capabilities

- `hello-world-example`: a minimal, framework-only page served from one Go file, with no live updates or client-side dependency.
- `chat-room-example`: the multi-feature chat app: lobby, rooms, SQLite storage, per-tab identity, Datastar SSE streaming, relay subjects, presence, typing, and command semantics.

### Modified Capabilities

None. The core capabilities are unchanged; this change adds example applications on top of them.

## Impact

- New directory `examples/hello-world/` in the root module.
- New module `examples/chat-room/` with its own `go.mod`, `go.sum`, app packages, tests, README, and embedded CSS.
- New dependency, limited to the chat-room module: `modernc.org/sqlite` (pure Go, no cgo).
- New documentation in `README.md` and `examples/chat-room/README.md`.
- New `Makefile` targets for running and testing the examples.
- No core API, behavior, or dependency changes. The counter example and the existing gate remain as they are.
- Reference code stays in the git-ignored `stario/` tree; nothing imports it.
