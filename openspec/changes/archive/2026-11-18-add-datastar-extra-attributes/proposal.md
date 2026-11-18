# Proposal

## Why

The `datastar` package covers event attributes, actions, initialization, signals,
binding, and the single-key reactive attributes, but the rest of the Datastar
attribute surface is still a raw string at every call site. Those names and
modifiers are invisible to the compiler, so a typo renders silently, and a view
that needs a map of reactive attributes has to assemble a JavaScript object by
hand. The reference implementation exposes all of these helpers; adding them
completes the attribute package and gives views one consistent entry point.

## What Changes

- Add map forms that render one JavaScript object:
  - `Attrs(map[string]string)` for `data-attr`.
  - `Classes(map[string]string)` for `data-class`.
  - `Styles(map[string]string)` for `data-style`.
  - `Computeds(map[string]string)` for `data-computed`, wrapping each value in a
    zero-argument function and expanding dotted keys into nested objects.
- Add event variants:
  - `OnIntersect(expression, opts...)` with threshold, once, exit, delay,
    debounce, throttle, and view transition modifiers.
  - `OnInterval(expression, opts...)` with a duration, leading, and view
    transition modifiers.
  - `OnSignalPatch(expression, opts...)` with delay, debounce, throttle, and
    signal include/exclude filters.
- Add the remaining non-event attributes:
  - `Ignore()` with `WithIgnoreSelf()`, and `IgnoreMorph()`, rendered without a
    value.
  - `Indicator(signal)` and `Ref(signal)`.
  - `Persist(opts...)` with signal filters, a storage key, and an optional
    session scope.
  - `PreserveAttr(names...)`.
  - `JSONSignals(opts...)` with signal filters and a terse form.
- Provide package-level helpers for the default prefix plus methods on
  `Attributes`, so a custom prefix applies to every attribute of the same
  instance.
- Validate keys, signal names, patterns, thresholds, and durations and panic on
  invalid input, matching the existing helpers.
- Add a multi-attribute node, because a signal patch with filters renders two
  attributes.
- Document the new helpers in the README and the package docs.
- Out of scope: Datastar Pro attributes other than `Persist` (`Animate`,
  `CustomValidity`, `MatchMedia`); `OnResize`; and migration of example views,
  which do not use these attributes today.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `datastar-attributes`: add requirements for map forms, viewport and interval
  events, signal patch filters, valueless attributes, indicator, ref, persist,
  preserve, and JSON signal rendering.

## Impact

- `datastar/attributes.go` gains the no-value attribute mode and the shared
  `Group` node; `datastar/reactive.go` gains the map forms, remaining
  attributes, and event variants.
- Tests extend `datastar/reactive_test.go`, `datastar/attributes_test.go`, and
  `datastar/interop_test.go`.
- README and `datastar/doc.go` gain examples for the new helpers.
- No new module dependency; no changes to `sse`, `signals`, or the response API.
