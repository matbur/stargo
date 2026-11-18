# Tasks

## 1. Rename and shared validation

- [x] 1.1 Rename the exported `Attr` result type to `Attribute` and update `On`, the package docs, and the existing tests; verify `go test ./datastar` passes and the rendered `data-on:click` bytes are unchanged.
- [x] 1.2 Add shared `validateAttributeKey` and `validateSignalName` helpers with the `datastar:` panic prefix; verify table tests accept lowercase kebab keys and dotted signal paths and panic on empty, uppercase, whitespace, quotes, `/`, `=`, non-ASCII, and empty path segments.

## 2. Signals

- [x] 2.1 Add dotted-key expansion and a JSON encoder for signal payloads that panics on conflicting paths and unsupported values; verify tests for nested `ui.open`, conflicting `ui` plus `ui.open`, empty segments, and deterministic key order.
- [x] 2.2 Add `Signals` and `WithIfMissing` for the default prefix and as an `Attributes` method; verify golden tests for `data-signals` and `data-signals__ifmissing`, escaped quotes, an empty payload, and parity between the package-level and instance forms.

## 3. Reactive attributes

- [x] 3.1 Add `Init` with `WithInitDelay` and `WithInitViewTransition`; verify tests for the plain form, each modifier, both modifiers, and digit-less durations panicking.
- [x] 3.2 Add `Bind` with `WithBindProp` and `WithBindEvent`; verify tests for a plain binding, each modifier, both modifiers in fixed order, and invalid signal names panicking.
- [x] 3.3 Add `Attr`, `Show`, `Text`, and `Effect`; verify golden tests for `data-attr:title`, `data-show`, `data-text`, and `data-effect`, escaped expression values, and invalid attribute keys panicking.
- [x] 3.4 Add `Class` and `Style`; verify golden tests for `data-class:hidden` and `data-style:width`, escaped expression values, and invalid class names or style properties panicking.
- [x] 3.5 Add `Computed` with signal-path expansion; verify a golden test for `data-computed="{'fullName':() => ($first + $last)}"`, nested names, and conflicting or empty names panicking.
- [x] 3.6 Verify every new helper composes with `gomponents` in a parent opening tag and that a page render and an SSE patch carry identical attribute bytes.

## 4. Example migration

- [x] 4.1 Replace the raw `data-init` attribute in `examples/hello/app/views.go` with `ds.Init`; verify the hello tests pass and the rendered attribute is unchanged.
- [x] 4.2 Replace the raw `data-init`, `data-signals__ifmissing`, `data-bind`, and `data-attr` attributes in the chat-room lobby views with `ds.Init`, `ds.Signals` with `WithIfMissing`, `ds.Bind`, and `ds.Attr`; verify lobby tests pass with the same rendered names and values, except the single-key `data-attr` map becoming the equivalent `data-attr:<key>` form.
- [x] 4.3 Replace the same raw attributes in the chat-room room views and remove any helper the migration makes unused; verify room tests pass with the same rendered names and values, except the single-key `data-attr` map becoming the equivalent `data-attr:<key>` form.
- [x] 4.4 Confirm no hand-built Datastar attribute remains in example views, excluding non-Datastar attributes such as `data-msg-id`; verify with a repository search.

## 5. Documentation

- [x] 5.1 Add a README subsection showing `ds.Init`, `ds.Signals` with `WithIfMissing`, `ds.Bind`, and `ds.Attr` through the `ds` import; state that signal values are JSON data while action payload and filter values remain trusted JavaScript; verify the snippets compile in a temporary test.

## 6. Integration checks

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`, and confirm all are clean.
- [x] 6.2 Run the hello and chat-room examples and confirm the increment, message send, typing, room creation, and room deletion behavior still works with the same Datastar attribute names.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm the updated `datastar-attributes` spec contains the new requirements.
