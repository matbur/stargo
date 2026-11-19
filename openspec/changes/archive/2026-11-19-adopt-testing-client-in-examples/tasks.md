# Tasks

## 1. Small examples

- [x] 1.1 Migrate `examples/hello-world/main_test.go` to `stargotest.NewClient` and response assertions; keep the page checks and the 404 check for `/live`.

## 2. Hello example

- [x] 2.1 Migrate `examples/hello/app/app_test.go` page checks to the client and assertion helpers; keep the link-resolution checks over `Response.Text()`.
- [x] 2.2 Replace the manual listener, HTTP client, scanner, and `readUntil` helper in the live test with `Client.Open` and `Stream.NextPatch`; verify the initial counter patch, the command request, and the updated counter patch.

## 3. Chat-room wiring and handlers

- [x] 3.1 Migrate `examples/chat-room/main_test.go` startup checks to the client and assertions; keep the missing-room redirect check.
- [x] 3.2 Add a client to the lobby `testApp` fixture and return `*stargotest.Response` from `do`; migrate status, header, and body checks without changing what each test proves.
- [x] 3.3 Replace the lobby live tests' manual server and `nextEvent` parser with `Stream.NextPatch`; verify the initial patch, the room-change patch, and the missing-identity navigation patch.
- [x] 3.4 Add a client to the room `handlerApp` fixture and return `*stargotest.Response` from `do`; migrate every status, header, and body check, including navigation-script responses.
- [x] 3.5 Replace the room live tests' manual server, readers, and `nextEvent` helpers with `Stream.NextPatch` and a local `nextPatchContaining` helper; keep join, message, typing, presence, deletion, and navigation coverage.

## 4. View tests

- [x] 4.1 Replace the local gomponents render helpers in the chat-room room, lobby, and page-shell view tests with `stargotest.Render`; keep all existing fragment assertions.

## 5. Verification

- [x] 5.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`; confirm all are clean.
- [x] 5.2 Run `openspec validate --changes --strict` and confirm the change passes with skipped specs.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review. It has no delta specs.
