# datastar-attributes Specification

## Purpose

Defines how Stargo renders Datastar attributes from typed inputs, including event handlers, initialization, signals, two-way and reactive bindings, modifier composition, escaping, name validation, and a configurable attribute prefix.

## Requirements

### Requirement: Event attribute rendering
A helper SHALL render an event handler as a single `data-on` attribute whose name carries the event and whose value is the supplied expression.

#### Scenario: Click handler
- **WHEN** a caller supplies the event `click` and the expression `@post('/command')`
- **THEN** the rendered attribute is `data-on:click="@post('/command')"`

#### Scenario: Expression is escaped
- **WHEN** the expression contains `"`, `'`, `&`, `<`, or `>`
- **THEN** the rendered value escapes each character as an HTML attribute value

#### Scenario: Expression is trusted JavaScript
- **WHEN** the expression contains signal references or JavaScript syntax
- **THEN** the helper writes it unchanged apart from HTML escaping

### Requirement: Modifier composition
The helper SHALL append each supplied modifier to the attribute name after the event, separated by double underscores.

#### Scenario: Prevent default
- **WHEN** a caller adds the prevent modifier to a `submit` event
- **THEN** the rendered attribute is `data-on:submit__prevent`

#### Scenario: Duration modifier
- **WHEN** a caller adds a debounce of 500 milliseconds to an `input` event
- **THEN** the rendered attribute is `data-on:input__debounce.500ms`

#### Scenario: Several modifiers
- **WHEN** a caller adds once, prevent, and a 300 millisecond throttle to a `click` event
- **THEN** all three modifiers appear in the attribute name

### Requirement: Canonical modifier order
The rendered modifier order SHALL be fixed regardless of the order in which the caller supplies modifiers.

#### Scenario: Reversed options
- **WHEN** the same modifiers are supplied in two different orders
- **THEN** both calls render byte-identical attribute names

#### Scenario: Defined order
- **WHEN** modifiers are present
- **THEN** the order is once, passive, capture, window or document or outside, prevent, stop, delay, debounce, throttle, view transition, case

### Requirement: Conflicting modifiers
The helper SHALL reject a combination that cannot behave as specified instead of rendering it.

#### Scenario: Passive with prevent
- **WHEN** a caller adds both passive and prevent to one event
- **THEN** the helper panics with a message naming the conflict

#### Scenario: Duplicate modifier
- **WHEN** a caller adds the same modifier more than once
- **THEN** the attribute name contains it once

### Requirement: Event name validation
The helper SHALL accept only lowercase kebab-case event names, so an event cannot inject additional attribute syntax.

#### Scenario: Valid custom event
- **WHEN** a caller supplies `my-event`
- **THEN** the rendered attribute is `data-on:my-event="<expression>"`

#### Scenario: Uppercase event
- **WHEN** a caller supplies `myEvent`
- **THEN** the helper panics with a message directing the caller to kebab-case

#### Scenario: Attribute injection
- **WHEN** an event name contains whitespace, a quote, `>`, `/`, or `=`
- **THEN** the helper panics instead of rendering the attribute

### Requirement: Attribute prefix
The helper SHALL render attributes with the prefix of the instance it belongs to, defaulting to `data-`.

#### Scenario: Default prefix
- **WHEN** a caller uses the default instance
- **THEN** a `click` event renders `data-on:click`

#### Scenario: Aliased bundle prefix
- **WHEN** a caller creates an instance with the prefix `data-star-`
- **THEN** a `click` event renders `data-star-on:click`

#### Scenario: Missing trailing hyphen
- **WHEN** an instance is created with `data-star` without the trailing hyphen
- **THEN** the prefix is normalized to `data-star-`

#### Scenario: Invalid prefix
- **WHEN** a prefix is empty or contains characters that cannot appear in an attribute name
- **THEN** construction panics

### Requirement: Renderer interoperability
The rendered value SHALL write itself to an `io.Writer`, so it composes with any HTML renderer that accepts values rendering to a writer.

#### Scenario: Composed with an HTML library
- **WHEN** a helper result is passed among other nodes to a renderer that accepts values implementing `Render(io.Writer) error`
- **THEN** the attribute appears in the parent element's opening tag

#### Scenario: Page and patch parity
- **WHEN** the same helper result is rendered in a page response and in an SSE element patch
- **THEN** both produce identical attribute bytes

### Requirement: Initialization attribute
A helper SHALL render one initialization attribute whose value is the supplied expression.

#### Scenario: Initialize on load
- **WHEN** a caller supplies the expression `@get('/live')`
- **THEN** the rendered attribute is `data-init="@get('/live')"`

#### Scenario: Expression is escaped
- **WHEN** the expression contains `"`, `'`, `&`, `<`, or `>`
- **THEN** the rendered value escapes each character as an HTML attribute value

### Requirement: Signals attribute
A helper SHALL render a signals payload as one JSON object in a `data-signals` attribute.

#### Scenario: Several signals
- **WHEN** a caller supplies the signals `count` with `0` and `open` with `false`
- **THEN** the rendered attribute is `data-signals="{\"count\":0,\"open\":false}"`

#### Scenario: If missing variant
- **WHEN** a caller requests the if-missing form
- **THEN** the attribute name is `data-signals__ifmissing`

#### Scenario: Deterministic key order
- **WHEN** the same signals are supplied in maps with different insertion order
- **THEN** both renders produce byte-identical attribute values

#### Scenario: Unsupported value
- **WHEN** a payload value cannot be encoded as JSON
- **THEN** the helper panics instead of rendering

### Requirement: Signal path expansion
A dotted key in a signals or computed payload SHALL render as a nested object, so the key addresses a nested signal path.

#### Scenario: Nested signal
- **WHEN** a caller supplies the key `ui.open`
- **THEN** the rendered object contains an `ui` object with an `open` entry

#### Scenario: Conflicting paths
- **WHEN** a payload sets both `ui` and `ui.open`
- **THEN** the helper panics instead of rendering

#### Scenario: Empty path segment
- **WHEN** a signal name is empty or contains an empty path segment
- **THEN** the helper panics

### Requirement: Two-way binding
A helper SHALL render a two-way binding attribute naming the bound signal, with optional property and event modifiers.

#### Scenario: Plain binding
- **WHEN** a caller binds the signal `message`
- **THEN** the rendered attribute is `data-bind="message"`

#### Scenario: Property and event
- **WHEN** a caller binds a signal with property `checked` and event `change`
- **THEN** the attribute name is `data-bind__prop.checked__event.change`

#### Scenario: Invalid signal name
- **WHEN** a bound signal name is empty or contains an invalid path segment
- **THEN** the helper panics

### Requirement: Attribute binding
A helper SHALL render one HTML attribute reactively as `data-attr:<key>` with a JavaScript expression as its value.

#### Scenario: Bound title
- **WHEN** a caller binds the attribute `title` to `$item.label`
- **THEN** the rendered attribute is `data-attr:title="$item.label"`

#### Scenario: Invalid attribute key
- **WHEN** the key is empty, uppercase, or contains whitespace or quotes
- **THEN** the helper panics

### Requirement: Reactive value attributes
Helpers SHALL render `data-show`, `data-text`, and `data-effect` attributes from supplied expressions.

#### Scenario: Conditional visibility
- **WHEN** a caller supplies `$error != null` to the show helper
- **THEN** the rendered attribute is `data-show="$error != null"`

#### Scenario: Bound text
- **WHEN** a caller supplies `$greeting` to the text helper
- **THEN** the rendered attribute is `data-text="$greeting"`

#### Scenario: Effect
- **WHEN** a caller supplies `el.focus()` to the effect helper
- **THEN** the rendered attribute is `data-effect="el.focus()"`

### Requirement: Reactive class attribute
A helper SHALL render one CSS class toggle as `data-class:<name>` with a JavaScript expression as its value.

#### Scenario: Toggle a class
- **WHEN** a caller toggles the class `hidden` with `!$expanded`
- **THEN** the rendered attribute is `data-class:hidden="!$expanded"`

#### Scenario: Invalid class name
- **WHEN** the class name is empty or cannot appear in an attribute name
- **THEN** the helper panics

### Requirement: Reactive style attribute
A helper SHALL render one inline style property as `data-style:<property>` with a JavaScript expression as its value.

#### Scenario: Reactive width
- **WHEN** a caller sets the property `width` with `$pct + '%'`
- **THEN** the rendered attribute is `data-style:width="$pct + '%'"`

#### Scenario: Invalid property
- **WHEN** the property is empty or contains characters that cannot appear in an attribute name
- **THEN** the helper panics

### Requirement: Computed signal attribute
A helper SHALL render one computed signal in a `data-computed` object, whose value is a zero-argument function returning the expression.

#### Scenario: Computed name
- **WHEN** a caller computes `fullName` from `$first + $last`
- **THEN** the rendered attribute value contains `'fullName':() => ($first + $last)`

#### Scenario: Invalid computed name
- **WHEN** the computed name is empty or contains an invalid path segment
- **THEN** the helper panics

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
