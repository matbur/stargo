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
