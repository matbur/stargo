# Tasks

## 1. Shared rendering

- [x] 1.1 Add the bare mode to `Attribute` and an internal constructor for valueless attributes; verify `data-ignore` renders without `=""`, valued attributes keep their current bytes, and `gofmt` is clean.
- [x] 1.2 Add `Group` with `Render` and `Type() g.NodeType`; verify it renders several attributes in order, reports the attribute node type, and composes inside a parent opening tag.

## 2. Map forms

- [x] 2.1 Add `jsObject` for string maps and nested maps with single-quoted keys, raw string values, and sorted keys; verify nested objects, empty maps, quoting, and deterministic bytes.
- [x] 2.2 Add `Attrs` with an `Attributes` method; verify the one-key and multi-key golden values, sorted order, escaped quotes in markup, invalid keys panicking, and prefix parity.
- [x] 2.3 Add `Classes` and `Styles` with `Attributes` methods; verify multi-key golden values, invalid names panicking, and escaped expression values.
- [x] 2.4 Add `Computeds`, refactor `Computed` to delegate to it, and verify arrow-function wrapping, sorted keys, nested dotted names, conflicting paths, and empty segments; confirm the existing `Computed` golden test is unchanged.

## 3. Event variants

- [x] 3.1 Add `OnIntersect` with threshold, once, exit, delay, debounce, throttle, and view transition options; verify the full modifier order, percent rounding, `half` and `full` keywords, duplicate threshold panics, out-of-range panics, and short durations panicking.
- [x] 3.2 Add `OnInterval` with duration, leading, and view transition options; verify the default form, `2s` and `1500ms` durations, implicit `duration.1s` with leading, view-transition-only form, and sub-millisecond panics.
- [x] 3.3 Add the shared filter object builder; verify include-only, exclude-only, combined order, and invalid patterns panicking.
- [x] 3.4 Add `OnSignalPatch` with timing and filter options; verify the single-attribute form, each timing modifier, the companion `data-on-signal-patch-filter` on a filtered group, and both attributes in order.

## 4. Remaining attributes

- [x] 4.1 Add `Ignore` with `WithIgnoreSelf()` and `IgnoreMorph`; verify both bare forms and self-only rendering, plus prefix parity.
- [x] 4.2 Add `Indicator` and `Ref`; verify golden values and invalid signal names panicking.
- [x] 4.3 Add `Persist` with include, exclude, key, and session options; verify the bare default, keyed session form, filters-only value, and invalid key panicking.
- [x] 4.4 Add `PreserveAttr`; verify space-joined names, uppercase and namespaced names accepted, and empty input or unsafe names panicking.
- [x] 4.5 Add `JSONSignals` with include, exclude, and terse options; verify the bare default, filtered value, terse name, terse-with-filters form, and prefix parity.

## 5. Documentation and interoperability

- [x] 5.1 Extend `interop_test.go` with a composed element using a map helper, a valueless attribute, and a filtered signal patch; verify the opening tag contents and page versus SSE patch byte parity.
- [x] 5.2 Add a README subsection covering the map forms, event variants, and remaining attributes through the `ds` import; state that map values and filter patterns are trusted JavaScript; verify the snippets compile in a temporary test.
- [x] 5.3 Update `datastar/doc.go` so the package overview lists the extended helpers and their trust boundary.

## 6. Integration checks

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`, and confirm all are clean.
- [x] 6.2 Run `openspec validate --changes --strict` and confirm the change and its delta spec pass.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm the updated `datastar-attributes` spec contains the new requirements.
