# Proposal

## Why

Stargo views currently build Datastar attributes from raw strings such as
`data-on:submit__prevent`. A typo in an attribute name is silent: Datastar ignores
unknown attributes, the page renders, and nothing reacts. Modifiers live inside the name,
so neither the compiler nor the reader can verify them, and `gomponents` does not escape
attribute names. Stario's `data.on(...)` avoids this; Stargo should offer an equivalent
typed helper. The same views also build action strings by hand, quoting paths with a local
`jsonString` helper and concatenating JavaScript, so action construction gets the same
treatment.

## What Changes

- Add a `datastar` package that renders Datastar attributes and fetch action strings from
  typed inputs.
- Provide `Default()` for the standard `data-` prefix and `New(prefix)` for an aliased
  bundle prefix; only `data-` is documented.
- Provide `On(event, expression, opts...)` producing `data-on:<event>` with canonical
  modifier order.
- Support these modifiers: once, passive, capture, stop, window, document, outside,
  delay, debounce, throttle, view transition, and case.
- Provide `Get`, `Post`, `Put`, `Patch`, and `Delete` action builders that quote the path
  as a JavaScript string literal and accept typed fetch options.
- Support fetch options: content type, selector, headers, open when hidden, payload,
  signal filters, retry policy and timings, and request cancellation.
- Validate event, modifier, prefix, path, and option inputs and panic on invalid or
  contradictory construction, matching the existing `route.New` precedent.
- Render escaped attribute values without importing an HTML component library; the result
  composes with `gomponents` and any other renderer structurally.
- Migrate `examples/hello` and `examples/chat-room` view code from raw `data-on*` and
  hand-built `@get`/`@post`/`@delete` strings to the helpers.
- **Not breaking**: raw attribute and action strings keep working. Action paths use one
  canonical single-quoted form, so chat-room responses change from `&#34;` to `&#39;`
  around action paths and their tests are updated.

Out of scope: `data-init`, `data-signals`, `data-bind`, `data-attr`, Datastar Pro actions
such as `@clipboard`, the Datastar script tag, and bundling or embedding the client
JavaScript.

## Capabilities

### New Capabilities

- `datastar-attributes`: construction and validation of Datastar `data-on` attributes
  from typed inputs, including modifiers and an optional prefix.
- `datastar-actions`: construction and validation of Datastar fetch action strings,
  including path quoting and typed fetch options.

### Modified Capabilities

None. The greeting example gains no new behavior, and chat-room behavior is unchanged even
though its rendered quoting is normalized.

## Impact

- New package `datastar` in the root module; examples and documentation import it as
  `ds`.
- `examples/hello/app/views.go` and the chat-room lobby and room views are updated to call
  the helpers; the room-local `jsonString` is removed and the lobby keeps it only for the
  delete confirmation text.
- Chat-room view tests that assert double-quoted action paths are updated to the canonical
  single-quoted form.
- New tests cover attribute output, action output, escaping, validation panics, prefix
  handling, and option rendering.
- No new module dependency; the package renders to an `io.Writer` and does not import
  `maragu.dev/gomponents` outside tests.
- No changes to `sse`, `signals`, or the response API.
