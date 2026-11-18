# Spec Delta

## ADDED Requirements

### Requirement: Attribute map rendering
A helper SHALL render a map of HTML attribute names to expressions as one `data-attr` object, where each value is a raw JavaScript expression.

#### Scenario: One mapped attribute
- **WHEN** a caller maps `open` to `sidebarOpen`
- **THEN** the rendered attribute is `data-attr="{'open':sidebarOpen}"`

#### Scenario: Several mapped attributes
- **WHEN** a caller maps `disabled` to `$busy` and `title` to `$label`
- **THEN** the object contains both entries and the keys are sorted alphabetically

#### Scenario: Deterministic key order
- **WHEN** the same entries are supplied in maps with different insertion order
- **THEN** both renders produce byte-identical attribute values

#### Scenario: Invalid mapped key
- **WHEN** an attribute name is empty, uppercase, or contains whitespace or quotes
- **THEN** the helper panics

### Requirement: Class map rendering
A helper SHALL render a map of class names to expressions as one `data-class` object.

#### Scenario: Several class toggles
- **WHEN** a caller maps `loading` to `$pending` and `hidden` to `!$open`
- **THEN** the rendered attribute is `data-class="{'hidden':!$open,'loading':$pending}"`

#### Scenario: Invalid class name
- **WHEN** a class name is empty or contains characters that cannot appear in an attribute name
- **THEN** the helper panics

### Requirement: Style map rendering
A helper SHALL render a map of style properties to expressions as one `data-style` object.

#### Scenario: Several reactive properties
- **WHEN** a caller maps `width` to `$pct + '%'` and `opacity` to `$fade`
- **THEN** the rendered attribute is `data-style="{'opacity':$fade,'width':$pct + '%'}"`

#### Scenario: Invalid style property
- **WHEN** a property is empty or contains characters that cannot appear in an attribute name
- **THEN** the helper panics

### Requirement: Computed map rendering
A helper SHALL render a map of computed signal names to expressions as one `data-computed` object whose entries are zero-argument functions returning the expression.

#### Scenario: Several computed signals
- **WHEN** a caller computes `total` from `$a + $b` and `ready` from `$count > 0`
- **THEN** each value is wrapped as `() => (<expression>)` and the keys are sorted alphabetically

#### Scenario: Nested computed path
- **WHEN** a caller computes `ui.total` from `$a + $b`
- **THEN** the rendered object contains an `ui` object with a `total` function entry

#### Scenario: Conflicting computed paths
- **WHEN** a payload sets both `ui` and `ui.total`
- **THEN** the helper panics instead of rendering

#### Scenario: Empty computed name
- **WHEN** a computed name is empty or contains an empty path segment
- **THEN** the helper panics

### Requirement: Viewport intersection attribute
A helper SHALL render a viewport intersection listener with optional threshold, once, exit, delay, debounce, throttle, and view transition modifiers in a fixed order.

#### Scenario: Threshold and once
- **WHEN** a caller supplies a threshold of 0.25, once, and a view transition
- **THEN** the rendered attribute is `data-on-intersect__threshold.25__once__viewtransition="<expression>"`

#### Scenario: Named threshold
- **WHEN** a caller supplies the `half` or `full` threshold keyword
- **THEN** the modifier is the bare keyword instead of a percentage

#### Scenario: Exit and durations
- **WHEN** a caller adds exit, a 100 millisecond delay, and a 200 millisecond debounce
- **THEN** the modifiers appear as `exit`, `delay.100ms`, and `debounce.200ms` after the threshold

#### Scenario: Threshold out of range
- **WHEN** a numeric threshold is below 0 or above 1
- **THEN** the helper panics

#### Scenario: Contradictory thresholds
- **WHEN** a caller supplies more than one threshold option
- **THEN** the helper panics

### Requirement: Interval attribute
A helper SHALL render an interval listener whose optional duration, leading, and view transition modifiers are composed into the attribute name.

#### Scenario: Default interval
- **WHEN** a caller supplies no options
- **THEN** the rendered attribute is `data-on-interval="<expression>"` and the interval is one second

#### Scenario: Custom duration and leading
- **WHEN** a caller supplies a duration of 2 seconds and leading
- **THEN** the rendered attribute is `data-on-interval__duration.2s.leading="<expression>"`

#### Scenario: Millisecond duration
- **WHEN** a caller supplies a duration of 1500 milliseconds
- **THEN** the duration modifier is `duration.1500ms`

#### Scenario: Default duration with leading
- **WHEN** a caller requests leading without a duration
- **THEN** the duration modifier is `duration.1s`

#### Scenario: View transition
- **WHEN** a caller adds a view transition without a custom duration
- **THEN** the attribute name is `data-on-interval__viewtransition`

#### Scenario: Duration below one millisecond
- **WHEN** a duration is shorter than one millisecond
- **THEN** the helper panics

### Requirement: Signal patch attribute
A helper SHALL render a signal patch listener with optional delay, debounce, and throttle modifiers in a fixed order.

#### Scenario: Debounced patch listener
- **WHEN** a caller supplies a 500 millisecond debounce
- **THEN** the rendered attribute is `data-on-signal-patch__debounce.500ms="<expression>"`

#### Scenario: Several modifiers
- **WHEN** a caller supplies a delay and a throttle
- **THEN** the delay appears before the throttle in the attribute name

#### Scenario: No modifiers
- **WHEN** a caller supplies no timing options
- **THEN** the rendered attribute is `data-on-signal-patch="<expression>"`

### Requirement: Signal patch filters
A signal patch helper SHALL render a companion `data-on-signal-patch-filter` attribute whose value is a filter object when include or exclude patterns are supplied.

#### Scenario: Include filter
- **WHEN** a caller includes the pattern `draft`
- **THEN** the companion attribute is `data-on-signal-patch-filter="{'include':'draft'}"`

#### Scenario: Include and exclude
- **WHEN** a caller supplies both an include and an exclude pattern
- **THEN** the filter object contains `include` before `exclude`

#### Scenario: No filters
- **WHEN** a caller supplies no patterns
- **THEN** no filter attribute is rendered

#### Scenario: Invalid pattern
- **WHEN** a pattern is empty, contains a control character, spans lines, or contains an unescaped `/`
- **THEN** the helper panics

### Requirement: Multi-attribute result
A helper that renders more than one attribute SHALL render each attribute in order inside the parent element's opening tag.

#### Scenario: Signal patch group with filters
- **WHEN** a filtered signal patch helper is composed with an HTML element
- **THEN** both the listener and the filter attribute appear in the opening tag

#### Scenario: Page and patch parity
- **WHEN** the same multi-attribute result is rendered in a page response and in an SSE element patch
- **THEN** both produce identical attribute bytes

### Requirement: Valueless attributes
A helper SHALL render a boolean attribute as its name alone, without an empty value assignment.

#### Scenario: Ignore
- **WHEN** a caller renders the ignore attribute
- **THEN** the rendered attribute is `data-ignore` with no `=""`

#### Scenario: Ignore only this element
- **WHEN** a caller requests the self-only form
- **THEN** the rendered attribute is `data-ignore__self` with no `=""`

#### Scenario: Ignore morphing
- **WHEN** a caller renders the ignore-morph attribute
- **THEN** the rendered attribute is `data-ignore-morph` with no `=""`

### Requirement: Indicator attribute
A helper SHALL render an indicator attribute naming the signal that tracks in-flight requests.

#### Scenario: Saving indicator
- **WHEN** a caller supplies the signal `saving`
- **THEN** the rendered attribute is `data-indicator="saving"`

#### Scenario: Invalid indicator signal
- **WHEN** the signal name is empty or contains an invalid path segment
- **THEN** the helper panics

### Requirement: Ref attribute
A helper SHALL render a ref attribute naming the signal that receives the current element.

#### Scenario: Search input ref
- **WHEN** a caller supplies the signal `searchInput`
- **THEN** the rendered attribute is `data-ref="searchInput"`

#### Scenario: Invalid ref signal
- **WHEN** the signal name is empty or contains an invalid path segment
- **THEN** the helper panics

### Requirement: Persist attribute
A helper SHALL render a persist attribute for local or session storage, with an optional storage key, optional signal filters, and an optional session scope.

#### Scenario: Plain persist
- **WHEN** a caller supplies no options
- **THEN** the rendered attribute is `data-persist` with no value

#### Scenario: Keyed session persist with filters
- **WHEN** a caller supplies the key `prefs`, the session scope, and an include pattern for `draft`
- **THEN** the rendered attribute is `data-persist:prefs__session="{'include':'draft'}"`

#### Scenario: Persist filters only
- **WHEN** a caller supplies an exclude pattern for `secret`
- **THEN** the rendered attribute is `data-persist="{'exclude':'secret'}"`

#### Scenario: Invalid storage key
- **WHEN** the storage key is empty or contains characters that cannot appear in an attribute name
- **THEN** the helper panics

### Requirement: Preserved attributes
A helper SHALL render a space-separated list of attribute names that DOM morphing must preserve.

#### Scenario: Two preserved attributes
- **WHEN** a caller supplies `data-testid` and `id`
- **THEN** the rendered attribute is `data-preserve-attr="data-testid id"`

#### Scenario: No attribute names
- **WHEN** a caller supplies no names
- **THEN** the helper panics

#### Scenario: Invalid attribute name
- **WHEN** a name is empty, contains whitespace, quotes, `=`, `<`, `>`, `/`, or `&`
- **THEN** the helper panics

### Requirement: JSON signals attribute
A helper SHALL render signals as JSON text, with optional include and exclude filters and an optional terse form.

#### Scenario: Plain JSON signals
- **WHEN** a caller supplies no options
- **THEN** the rendered attribute is `data-json-signals` with no value

#### Scenario: Filtered JSON signals
- **WHEN** a caller includes the pattern `email|password`
- **THEN** the rendered attribute is `data-json-signals="{'include':'email|password'}"`

#### Scenario: Terse JSON signals
- **WHEN** a caller requests the terse form
- **THEN** the attribute name is `data-json-signals__terse`

#### Scenario: Terse form with filters
- **WHEN** a caller requests the terse form and an exclude pattern
- **THEN** the attribute name is `data-json-signals__terse` and the value is the filter object

### Requirement: Prefix behavior of the extended helpers
Every extended helper SHALL use the prefix of the instance it belongs to, with package-level helpers using the default prefix.

#### Scenario: Aliased bundle prefix
- **WHEN** an instance is created with the prefix `data-star-`
- **THEN** map, event variant, and valueless helpers render `data-star-attr`, `data-star-on-interval`, and `data-star-ignore`

#### Scenario: Invalid input before prefix use
- **WHEN** an extended helper receives an invalid key, signal name, pattern, threshold, or duration
- **THEN** it panics instead of rendering the attribute
