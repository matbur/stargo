# Tasks

## 1. Hello world example

- [ ] 1.1 Add `examples/hello-world/main.go` with the gomponents document, one `route.Get("/")` handler, `stargo.New`, and `ListenAndServe(":8080")`; verify with `go build ./examples/hello-world`.
- [ ] 1.2 Add `examples/hello-world/main_test.go` asserting the doctype, title, `h1`, paragraph, absence of `script` and Datastar attributes, and `404` for `GET /live`; verify with `go test ./examples/hello-world`.
- [ ] 1.3 Add `examples/hello-world/README.md` with the run command and the no-Datastar note; verify the documented command builds and serves the page.

## 2. Chat room module and persistence

- [ ] 2.1 Create `examples/chat-room/go.mod` and `go.sum` with the `replace` directive, `modernc.org/sqlite`, and `app/config` reading `CHAT_DB_PATH` with the `:memory:` default; verify `go build ./...` inside the module and a config test for the default and override.
- [ ] 2.2 Add `app/db` with `Open`, connection limits, and idempotent schema application; verify a test opens an in-memory database, applies the schema twice, and queries it.
- [ ] 2.3 Add `app/features/room` models, schema, and room queries (`list`, `get`, `add` with unique slugs, `delete` with cascade); verify table-driven tests for ordering, slug collisions, and cascade deletion.
- [ ] 2.4 Add message and presence queries: insert with 100-message trim, ordered history, users per room, counts by room, and typing changes that report whether the value changed; verify each query with tests.

## 3. Shared web layer

- [ ] 3.1 Add the `app/routes` leaf package with lobby, room, live, send, typing, create, and delete handles; verify that methods, patterns, and `Href` output match the registered paths.
- [ ] 3.2 Add `app/common` identity: eight-character mint, adjective-animal name, color, query override, and signal reader; verify distinct ids, format, query precedence, and incomplete-signal rejection with tests.
- [ ] 3.3 Add the `app/common` page shell with embedded CSS, pinned Datastar module URL, and body slot; verify the shell test finds the inline style, no stylesheet link, the versioned script URL, and the supplied body.

## 4. Lobby feature

- [ ] 4.1 Add lobby views for the live patch, room cards, empty state, create dialog, signals, and init script; verify tests for the empty state, room metadata, online counts, and the `#lobby-live` patch root.
- [ ] 4.2 Add lobby handlers `show`, `create`, and `delete` with length limits, default description, SSE navigation, and relay notifications; verify `GET /`, creation, deletion, missing-room deletion, and empty-title cases with `httptest`.
- [ ] 4.3 Add the lobby live handler with identity validation, the filtered subscription, and the initial patch; verify with a real listener that an open stream receives `#lobby-live`, that creating a room pushes another patch, and that a missing identity navigates to the lobby.

## 5. Room feature

- [ ] 5.1 Add room views for messages, typing text, online avatars, the input form, and the live patch; verify tests for own-message styling, the typing summary variants, history ordering, and the `#room-live` patch root.
- [ ] 5.2 Add room `show`, `send`, and `typing` handlers with unknown-room redirects, user-presence checks, message limits, and relay notifications; verify page rendering, redirect, stored message, empty message, and typing change behavior with `httptest`.
- [ ] 5.3 Add the room live handler that subscribes before presence, patches on room events, navigates to the lobby on deletion, and cleans up presence on exit; verify with a real listener that join, message, typing, deletion, and disconnect each produce the expected patch or navigation.
- [ ] 5.4 Verify live rejection paths: an unknown room and a request without identity each receive an SSE navigation and the handler ends without registering presence.

## 6. Application wiring and example documentation

- [ ] 6.1 Add `examples/chat-room/main.go` that reads config, opens the database, applies the schema, builds the relay, registers both pages, and serves on `:8080`; verify `go build ./...` and that the startup path works with an in-memory database.
- [ ] 6.2 Add `examples/chat-room/README.md` covering run, test, layout, routes, persistence, environment, presence lifetime, and the Datastar CDN requirement; verify every documented command runs as written.
- [ ] 6.3 Document both examples in the root `README.md` and add `run-hello-world`, `run-chat-room`, and `test-examples` targets to the `Makefile`; verify each target where it can run non-interactively.

## 7. Verification

- [ ] 7.1 Run the root gate `go vet ./...`, `go test -race ./...`, and `go build ./...`; verify all pass.
- [ ] 7.2 Run the chat-room gate `go vet ./...`, `go test -race ./...`, and `go build ./...` inside `examples/chat-room`; verify all pass.
- [ ] 7.3 Run `openspec validate stargo-examples --strict`; verify the change is valid.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review.
