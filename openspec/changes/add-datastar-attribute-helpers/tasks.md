# Tasks

## 1. Package and attribute rendering

- [ ] 1.1 Create the `datastar` package with an `Attributes` value, `Default()`, `New(prefix)`, a package-level `On`, and an attribute renderer implementing `Render(io.Writer) error`; verify `go build ./datastar` and a unit test asserting `data-on:click="@post('/command')"`.
- [ ] 1.2 Implement attribute-value escaping with the same replacements as `gomponents`; verify a table test comparing the helper output with a `gomponents` attribute node for NUL, `"`, `'`, `&`, `<`, and `>`.
- [ ] 1.3 Write the package doc comment describing default usage through the `ds` import alias, modifiers, actions, and the optional prefix; verify `go doc ./datastar` shows the documented entry points.

## 2. Attribute modifiers

- [ ] 2.1 Add the option config and constructors `WithOnce`, `WithPassive`, `WithCapture`, `WithStop`, `WithWindow`, `WithDocument`, `WithOutside`, `WithDelay`, `WithDebounce`, `WithThrottle`, `WithViewTransition`, and `WithCase`; verify one table test per modifier against the expected attribute name.
- [ ] 2.2 Render modifiers in the fixed order from the design and collapse duplicates; verify a test supplying the same modifiers in two orders produces identical bytes and a repeated `WithOnce` yields one `__once`.
- [ ] 2.3 Panic on `passive` with `prevent`, an unsupported case value, and a non-positive delay, debounce, or throttle; verify each case with a `recover` test that checks the panic message prefix.

## 3. Attribute name validation

- [ ] 3.1 Enforce the lowercase kebab event grammar; verify tests for valid `my-event` and panics for `myEvent`, whitespace, quotes, `>`, `/`, `=`, leading or trailing hyphen, underscore, and non-ASCII input.
- [ ] 3.2 Normalize and validate the prefix as `data-<alias>-`; verify tests for the default, `data-star-`, a missing trailing hyphen, an empty alias, uppercase, and characters that cannot appear in an attribute name.

## 4. Renderer interoperability

- [ ] 4.1 Verify the attribute helper result composes among other nodes in `gomponents` and appears inside the parent opening tag, using a test-only import of the library.
- [ ] 4.2 Verify page and SSE patch parity by rendering the same attribute value through the full-page path and an element patch and comparing the attribute bytes.

## 5. Action builders

- [ ] 5.1 Add `Get`, `Post`, `Put`, `Patch`, and `Delete` returning expression strings and a shared single-quote JavaScript literal encoder; verify one test per verb and a quoting table for `'`, `\`, newline, carriage return, U+2028, U+2029, and a NUL panic.
- [ ] 5.2 Add `ActionOption` constructors `WithContentType`, `WithForm`, `WithSelector`, `WithHeaders`, `WithOpenWhenHidden`, `WithPayload`, `WithSignalsInclude`, `WithSignalsExclude`, `WithRetry`, `WithRetryInterval`, `WithRetryScaler`, `WithRetryMaxWait`, `WithRetryMaxCount`, and `WithRequestCancellation`; verify each renders the expected option pair.
- [ ] 5.3 Render option objects in the fixed design order, omit the object when no options are supplied, and produce identical strings for reordered options; verify with golden-string tests.
- [ ] 5.4 Panic on an unknown content type, a selector without the form content type, an invalid header name, a non-positive retry duration, a non-finite or non-positive scaler, a negative retry count, and an unknown retry or cancellation mode; verify one `recover` test per case.
- [ ] 5.5 Render signal filters as regex literals and panic on an empty pattern, an unescaped `/`, or a line break; verify include-only, include-and-exclude, and rejection tests.
- [ ] 5.6 Verify a POST action composes with `On` and that HTML escaping turns its quotes into `&#39;`; verify the composed attribute is identical in a page render and an SSE patch.

## 6. Example migration

- [ ] 6.1 Replace `data-on:click` and the `@post('/command')` expression in `examples/hello/app/views.go` with `On` and `Post`, and replace the `data-init` value with `Get`, importing the package as `ds`; verify the example tests still pass and the rendered attribute names are unchanged.
- [ ] 6.2 Replace `@get`, `@post`, and `@delete` strings in the chat-room lobby views with the action helpers through the `ds` import, including `WithRetry`; keep the local string helper only for the delete confirmation text, and verify lobby tests pass with the single-quoted expectations.
- [ ] 6.3 Replace `@get`, `@post`, and `@post('/typing')` strings in the chat-room room views with the action helpers through the `ds` import, remove the now-unused local `jsonString`, and verify room tests pass with the single-quoted expectations.
- [ ] 6.4 Confirm no `"data-on` string and no hand-built `"@get("`, `"@post("`, or `"@delete("` remains in example views; verify with a repository search.

## 7. Documentation

- [ ] 7.1 Add a README subsection showing `On` with modifiers, an action inside `On`, and a fetch option, all through the `ds` import; state that the prefix must match across pages, SSE patches, and raw HTML, and that payload and filter values are trusted JavaScript; verify the snippets compile in a temporary test.

## 8. Integration checks

- [ ] 8.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` and confirm all are clean.
- [ ] 8.2 Run the hello example and the chat-room example and confirm increment, message send, typing, room creation, and room deletion still work with the same Datastar attribute names.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm both `datastar-attributes` and `datastar-actions` specs appear under `openspec/specs/`.
