# Design

## Context

See `proposal.md` for motivation. Current constraints:

- `Attributes` carries the prefix, and package-level helpers call `Default()`, so
  every helper has one implementation.
- `Attribute` renders exactly one valued attribute and marks itself as a
  `gomponents` attribute; nothing can render two attributes yet.
- `escapeAttribute` mirrors `gomponents` and escapes `'` to `&#39;`; the source
  value uses single quotes, so tests assert escaped bytes.
- `validateAttributeKey` accepts lowercase kebab-case, `validateSignalName`
  accepts dot-separated paths, and `validatePattern` accepts one line with an
  escaped `/`.
- `expandSignalPaths` already turns dotted keys into nested maps and panics on
  conflicts.
- `sortedKeys` exists in `actions.go` and is reused for deterministic maps.
- Datastar's own defaults are one second for `data-on-interval` and no value
  for the boolean attributes, so matching them matters for compatibility.

## Goals / Non-Goals

**Goals:**

- Typed helpers for every remaining non-Pro-except-`Persist` attribute.
- Byte-identical output regardless of map insertion order.
- One shared renderer for valued, valueless, and multi-attribute results.
- Validation that fails at startup with a `datastar:` panic message.

**Non-Goals:**

- Datastar Pro attributes other than `Persist` (`Animate`, `CustomValidity`,
  `MatchMedia`) and `OnResize`.
- Parsing or sandboxing JavaScript expressions; they stay trusted input.
- New escaping rules; the existing attribute escaper is reused.
- Migration of example views, which do not use these attributes today.

## Decisions

### Map values are expressions, keys are data

`Attrs`, `Classes`, `Styles`, and `Computeds` take `map[string]string`. Keys are
quoted as JavaScript string literals with `quoteJSString`, so a key cannot
break out of the object. Values are written verbatim as JavaScript expressions,
matching `Attr` and `Class` and the reference implementation. The doc comments
state that values must never carry user-controlled text.

Alternative: a typed expression type with helpers such as `ds.JS("...")`. That
would be safer but would make every call site noisier and diverge from the rest
of the package; the package already treats expressions as trusted input.

### Sorted keys for deterministic output

Every map form iterates `sortedKeys`, so output depends only on the map contents.
The reference implementation preserves insertion order; Go maps have none, and
determinism is worth more than matching a Python implementation detail. SSE
parity tests then compare bytes without normalizing order.

### A shared JavaScript object encoder

`jsObject(map[string]any)` renders `{...}` with single-quoted keys and raw string
values, and recurses into nested maps. `Attrs`, `Classes`, and `Styles` pass
their maps through it; `Computeds` first expands signal paths and wraps each
leaf in `() => (<expression>)`. This deliberately differs from `Signals`, which
encodes JSON data, because these objects contain functions and expressions.

An empty map renders `{}` instead of panicking: the caller supplied a valid
object; whether it is useful is a view concern.

### `Computeds` reuses `Computed` semantics

`Computed` already expands one dotted name into a nested object. `Computeds`
builds `map[string]any{name: "() => (" + expr + ")"}` and reuses
`expandSignalPaths`, so `ui.total` nests and conflicts panic identically.
`Computed(name, expr)` becomes a one-entry `Computeds` call internally to keep a
single formatting path.

### A `Group` node for multi-attribute helpers

`OnSignalPatch` can render two attributes. It returns `Group`, a named slice of
`Attribute` with `Render` and `Type() g.NodeType` returning
`g.AttributeType`. Rendering writes each attribute in order, and `gomponents`
places the group inside the opening tag like any other attribute node.
Alternative: return `[]g.Node`; rejected because it loses the attribute marker
and makes the result harder to type.

`OnSignalPatch` always returns a `Group`, even without filters, so the return
type does not change with options.

### Valueless attributes on `Attribute`

`Attribute` gains a `bare` flag set by an internal constructor. `Render` writes
only ` name` for a bare attribute. `Ignore`, `IgnoreMorph`, bare `Persist`, and
bare `JSONSignals` use it. The zero value keeps the valued behavior, so existing
constructors are unchanged.

### Per-helper option types with prefixed names

`OnOption` already uses names such as `WithOnce`, `WithDelay`, and
`WithViewTransition`. Reusing those names would collide, so each new event
variant gets its own option type and prefixed constructors:
`WithIntersectThreshold`, `WithIntersectOnce`, `WithIntervalDuration`,
`WithSignalPatchDebounce`, `WithPersistKey`, `WithJSONSignalsTerse`, and so on.
The prefixed names also read better when a call mixes families.

### Intersection thresholds

`WithIntersectThreshold(fraction float64)` accepts `0..1` and renders
`threshold.<percent>` with `math.Round(fraction*100)`, matching the reference.
`WithIntersectHalf()` and `WithIntersectFull()` render the bare keyword, and
`WithIntersectThresholdKeyword(keyword string)` accepts a validated
kebab-case keyword such as `any` for future Datastar values. Supplying more than
one threshold option panics, because the rendered attribute cannot express both.

### Interval durations

A duration renders as whole seconds when it divides evenly by one second
(`2s`), otherwise as milliseconds (`500ms`, `1500ms`). One second is Datastar's
default and is omitted unless leading is requested, in which case the reference
renders `duration.1s.leading`. Durations below one millisecond panic, sharing the
rule already used by `milliseconds`.

`WithIntervalDuration(time.Second)` with no leading therefore still renders the
plain attribute; this avoids changing behavior for a value that is already the
default.

### Signal filters share one builder

Include and exclude patterns are validated with `validatePattern` and rendered
as `{'include':'...'}`, `{'exclude':'...'}`, or
`{'include':'...','exclude':'...'}`. `OnSignalPatch`, `Persist`, and
`JSONSignals` reuse one helper, so order, quoting, and validation cannot drift.
Patterns are trusted regular expressions; callers join literal names with `|`
themselves, as they already do for actions.

### Filtered signal patch returns the companion attribute

The listener attribute and `data-on-signal-patch-filter` are separate attributes
in Datastar, so the helper renders both in one `Group`. Without filters, the
group holds one attribute. `data-on-signal-patch-filter` does not take the
timing modifiers; only the listener name carries them.

### `PreserveAttr` validation differs from attribute keys

Name lists are space-separated, so a name containing whitespace would change
the meaning. Each name is therefore rejected when empty or when it contains
whitespace, control characters, `"`, `'`, `=`, `<`, `>`, `/`, or `&`. Unlike
`validateAttributeKey`, uppercase, `:`, and `_` are allowed, because SVG names
such as `viewBox` and namespaced attributes are legitimate preservation targets.
`PreserveAttr()` with no names panics instead of rendering an empty value.

### Placement

- `maps.go`: `Attrs`, `Classes`, `Styles`, `Computeds`, and `jsObject`.
- `handlers.go`: `OnIntersect`, `OnInterval`, `OnSignalPatch`, and their options.
- `extras.go`: `Ignore`, `IgnoreMorph`, `Indicator`, `Ref`, `Persist`,
  `PreserveAttr`, `JSONSignals`, and the shared filter builder.
- `attributes.go`: `Attribute` bare mode and the `Group` type.

Tests follow the same split, and `interop_test.go` gains composition and page
versus patch parity checks for a valueless attribute and a filtered signal
patch.
