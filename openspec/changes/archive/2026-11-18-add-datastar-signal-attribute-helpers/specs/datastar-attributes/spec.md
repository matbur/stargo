# Spec Delta

## ADDED Requirements

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
