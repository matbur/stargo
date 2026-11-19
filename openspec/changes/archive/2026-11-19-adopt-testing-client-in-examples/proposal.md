# Proposal

## Why

The new `testing` package serves an app on a loopback server, keeps cookies,
encodes forms, asserts responses, and parses Datastar patches. The example tests
predate it: they still build `httptest` recorders and servers, parse SSE frames
line by line, and match raw strings. That duplicates plumbing, hides intent, and
leaves the package's public API unexercised in the examples users copy.

## What Changes

- Migrate app-level HTTP tests in `examples/hello-world`, `examples/hello`, and
  `examples/chat-room` to `stargotest.NewClient`, response assertions, and typed
  element and signal patches.
- Replace custom SSE readers, event scanners, and `nextEvent` helpers with
  `Client.Open` plus `Stream.NextPatch`.
- Replace local gomponents render helpers in example view tests with
  `stargotest.Render`.
- Keep direct request-parsing tests that do not run an app on `httptest`; they
  test a function, not the HTTP server path.
- Remove the now-unused test scaffolding from the examples.
- No production code changes and no capability behavior changes.

## Capabilities

### New Capabilities

None. This is a test-only refactor; the change declares `skip_specs: true`.

### Modified Capabilities

None.

## Impact

- Test files under `examples/hello-world`, `examples/hello`, and
  `examples/chat-room`.
- No changes to framework packages, example handlers, routes, or templates.
- Existing coverage stays: page rendering, redirects, form and JSON handlers,
  SSE element patches, navigation patches, and presence updates.
