# Proposal

## Why

The `datastar` package renders event attributes and actions, but the examples still
build every other Datastar attribute by hand: `data-init`, `data-signals__ifmissing`,
`data-bind`, and `data-attr` are raw strings. Those names and modifiers are invisible to
the compiler, so a typo still renders silently. The reference implementation exposes the
full attribute surface through typed helpers; adding the state and binding helpers closes
the remaining hand-built attributes in the examples and gives views one consistent entry
point.

## What Changes

- Add typed helpers for non-event Datastar attributes:
  - `Init(expression)` for `data-init`.
  - `Signals(payload, opts...)` for `data-signals` JSON, with `WithIfMissing()` for
    `data-signals__ifmissing`.
  - `Bind(signal, opts...)` for `data-bind`, with `WithBindProp` and `WithBindEvent`.
  - `Attr(key, expression)` for `data-attr:<key>`.
  - `Show(expression)`, `Text(expression)`, and `Effect(expression)` for their
    single-value attributes.
  - `Class(name, expression)` for `data-class:<name>`.
  - `Style(property, expression)` for `data-style:<property>`.
  - `Computed(name, expression)` for one `data-computed` entry.
- Provide package-level helpers for the default prefix plus methods on `Attributes`, so a
  custom prefix applies to every attribute of the same instance.
- Validate attribute keys and signal names and panic on invalid input, matching `On`.
- Expand dotted keys in `Signals` and `Computed` into nested signal objects, so a path
  such as `ui.open` reaches `$ui.open`.
- **BREAKING**: rename the exported `Attr` result type to `Attribute`, so the package can
  expose an `Attr` helper for `data-attr`. The package is unreleased; the rename affects
  only its own tests and examples.
- Migrate the remaining raw attributes in the hello and chat-room examples and document
  the new helpers in the README.
- Out of scope: plural mapping forms for `data-attr`, `data-class`, `data-style`, and
  `data-computed`; `data-ignore`, `data-ignore-morph`, `data-indicator`, `data-ref`,
  `data-persist`, `data-preserve-attr`, `data-json-signals`; `data-on-intersect`,
  `data-on-interval`, and `data-on-signal-patch`; Datastar Pro attributes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `datastar-attributes`: add requirements for initialization, signals, binding, and
  reactive single-value attributes, including their validation and sheet quoting.

## Impact

- New files under `datastar/` for the attribute families, a shared JSON object encoder,
  and signal-path expansion; tests extend `datastar/attributes_test.go` and add focused
  files for signals and reactive attributes.
- `datastar/attributes.go` renames `Attr` to `Attribute`; `On` keeps its behavior.
- `examples/hello/app/views.go` and the chat-room lobby and room views replace raw
  `data-init`, `data-signals__ifmissing`, `data-bind`, and `data-attr` attributes.
- README gains examples for binding, signals, and a reactive attribute.
- No new module dependency; no changes to `sse`, `signals`, or the response API.
