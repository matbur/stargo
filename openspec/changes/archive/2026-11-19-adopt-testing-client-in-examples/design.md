# Design

## Context

The `testing` package is available in the root module. The example applications
live in the same module (`examples/hello-world`, `examples/hello`) and in a
separate module (`examples/chat-room`) that already replaces the root module with
`../..`. No import changes are needed beyond adding
`stargotest "github.com/matbur/stargo/testing"`.

Current test patterns:

- `examples/hello-world/main_test.go` builds a recorder per request.
- `examples/hello/app/app_test.go` builds a recorder and starts a real server to
  read the live stream manually.
- `examples/chat-room/main_test.go` builds recorders for the startup check.
- `lobby/handlers_test.go` and `room/handlers_test.go` wrap recorders in `do`
  helpers.
- `lobby/handlers_test.go`, `room/live_test.go`, and the hello live test start
  `httptest.NewServer`, read SSE lines, and match substrings.
- View tests build local render helpers around gomponents.

## Goals / Non-Goals

**Goals:**

- Examples show the package's intended usage.
- Remove duplicated server, recorder, and SSE-frame code.
- Keep every existing assertion or replace it with a stronger typed check.

**Non-Goals:**

- Change example application behavior or templates.
- Migrate pure request-parsing tests that never start an app.
- Add new testing-package features.

## Decisions

### One client per app fixture

Each `testApp` / `handlerApp` fixture registers routes first, then creates
`stargotest.NewClient(t, app)`. The client starts its own loopback server and
registers cleanup. Tests call `client.Do(client.Request(...))` instead of
`app.ServeHTTP(recorder, ...)`.

The `do` helpers stay, but return `*stargotest.Response` and attach
`application/json` to non-empty request bodies. They preserve the call sites'
shape while delegating transport to the client.

### Assertions replace recorder field access

`assertStatus`, `assertHeader`, `assertContains`, and `assertNotContains` chains
replace `recorder.Code`, `recorder.Header().Get`, and `strings.Contains` loops.
Checks that need the body for further processing use `Response.Text()`.

### Typed patches replace SSE frame parsing

`Client.Open` returns a `Stream`; `NextPatch` skips pings and returns typed
`Patch` values. Tests assert on `Patch.Kind`, `Patch.Selector`, and
`Patch.Elements`. Navigation assertions check the script in `Patch.Elements`
instead of raw event text.

Streaming requests get a context deadline where the old test had one, so a
missing patch fails the test through the client instead of hanging.

### Keep direct parsing tests on `httptest`

`common/identity_test.go` calls `common.ForPage` and `common.FromSignals` with a
bare request. There is no handler or server to test, so `httptest.NewRequest`
remains the right tool. The same applies to unit tests of view output; they use
`Render`, not a client.

### The hello live test no longer proves `Serve`

The old test started `net.Listen` and called `application.Serve` to obtain a
stream. The framework's `Serve` lifecycle is covered by root tests. The example
test's purpose is the live counter patch, so it moves to the client. This drops
no unique production coverage.

### Placement

Changes stay inside the example test files. No new helper package and no
production edits.
