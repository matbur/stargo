# Design

## Context

See `proposal.md` for motivation. Current constraints:

- `datastar` already provides `On` plus fetch actions, with `Attributes` carrying the
  prefix and package-level helpers using the default prefix.
- `Attr` is the exported result type rendered by `Render(io.Writer) error` and marked as
  a `gomponents` attribute. A package-level `Attr` helper cannot exist while that name
  belongs to the type.
- `gomponents` escapes only values, so every new attribute name must be validated before
  rendering.
- Datastar signal JSON patches treat each object key literally; `{"ui.open": true}`
  creates a signal that `$ui.open` cannot reach.
- The reference implementation renders reactive attributes as one attribute per key, with
  a plural map form as an additional convenience.

## Goals / Non-Goals

**Goals:**

- One attribute family API for initialization, signals, binding, and reactive values,
  sharing prefix, escaping, and validation behavior with `On`.
- Deterministic output for maps, so golden tests and SSE parity stay stable.
- Signal names that work with Datastar's nested paths.

**Non-Goals:**

- Plural map forms for `data-attr`, `data-class`, `data-style`, and `data-computed`.
- Modifiers for `data-init` beyond an optional delay and view transition.
- Signal patch filtering, event variants such as `data-on-intersect`, and Pro attributes.
- Parsing or interpreting JavaScript expressions; they remain trusted input.

## Decisions

### Extend `datastar-attributes`, not a new capability

The new helpers have the same contract as `On`: prefix from the instance, HTML escaping,
name validation, panic on programmer error. A separate capability would duplicate those
requirements. The delta adds requirements to `datastar-attributes`; its Purpose in the
main spec stays focused on event attributes, so the archive updates it to cover the whole
attribute surface.

### Rename the result type to `Attribute`

`Attr` becomes the helper for `data-attr:<key>`, matching the reference naming and the
attribute it renders. Keeping the type named `Attr` would force an inconsistent helper
name such as `DataAttr` or `Data`. The package is unreleased, so the rename costs only
its own tests. Alternative: keep the type and name the helper `AttrFor`; rejected because
the exported API would explain the workaround at every call site.

### New files by attribute family

- `attributes.go` keeps `On`, `Attributes`, modifiers, and the shared renderer.
- `reactive.go` holds `Init`, `Bind`, `Attr`, `Show`, `Text`, `Class`, `Style`, `Effect`,
  and `Computed`, plus their options.
- `signals.go` holds `Signals`, `WithIfMissing`, JSON encoding, and signal-path
  expansion.

Package-level wrappers call the `Default()` methods, as `On` does, so the two entry
points cannot drift.

### Signals payloads are data, not JavaScript

`Signals` accepts `map[string]any` and encodes it with `encoding/json`. Values are JSON
data, never evaluated as JavaScript. This differs from action payloads, whose values are
trusted expressions; the doc comment states the distinction. `encoding/json` sorts map
keys, which gives deterministic output.

The encoded JSON is placed into the attribute value and escaped by the existing renderer,
so `"` becomes `&#34;` in markup and decodes back before Datastar parses it.

### Expand dotted signal paths

`Signals` and `Computed` expand `a.b` into `{"a":{"b":...}}` before encoding. A key used
both as a leaf and as a path prefix panics, as does an empty segment. Expansion is shared
between the two helpers. Alternative: pass keys through and document that dotted keys are
literal; rejected because it silently produces unreachable signals.

### Single-key reactive attributes

`Attr`, `Class`, `Style`, and `Computed` take one key each. Callers needing several keys
call the helper several times or keep `g.Attr` for the map form. This keeps the API small
and the validation strict. Alternative: add plural map forms now; deferred until a view
needs them.

### Options per attribute family

- `SignalsOption` with `WithIfMissing()`.
- `BindOption` with `WithBindProp(prop)` and `WithBindEvent(event)`, rendered in that
  order.
- `InitOption` with `WithInitDelay(d)` and `WithInitViewTransition()`, rendered in that
  order.

Names are family-scoped because bare `WithDelay` and `WithViewTransition` already belong
to `OnOption`. Alternative: reuse `OnOption` and ignore unsupported values; rejected
because a misplaced option would be silent.

### Validation shared with `On`

One `validateAttributeKey` accepts lowercase kebab-case names and rejects empty names,
uppercase, whitespace, quotes, `/`, `=`, and non-ASCII. `Bind`, `Signals`, and `Computed`
use `validateSignalName`, which splits on `.` and rejects empty segments and characters
that cannot appear in a signal path. All panics use the `datastar:` prefix.

### Computed wire format

`Computed` renders `{'name':() => (expression)}` as a JavaScript object literal, matching
the reference implementation. The attribute renderer escapes `'` and `>` in markup;
Datastar receives the decoded `=>` arrow. Alternative: JSON with a string expression;
rejected because Datastar expects a function value.

## Risks / Trade-offs

- Attribute values visibly contain `&#39;` and `&gt;` for computed signals → golden tests
  assert decoded semantics through the same renderer, and the README shows the rendered
  form.
- Renaming `Attr` breaks any code outside this repository that imported the package →
  acceptable while the package is unreleased; the change is recorded as **BREAKING**.
- `encoding/json` rejects values such as channels and functions at render time → panic is
  the intended programmer-error behavior, and the message names the payload.
- Dotted-path expansion changes key shape on the wire → tests cover nested and conflicting
  paths, and the behavior matches the reference implementation.

## Migration Plan

1. Rename the type and update `On` and its tests.
2. Add the new helpers and tests.
3. Replace raw attributes in the examples; update their assertions.
4. Update the README.

Rollback is a revert of the change: the new helpers are additive, and the rename touches
only package-internal call sites and tests.

## Open Questions

None.
