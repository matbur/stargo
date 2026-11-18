# datastar-attributes Specification

## Purpose

Defines how Stargo renders Datastar event attributes from typed inputs, including modifier composition, escaping, name validation, and a configurable attribute prefix.

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
