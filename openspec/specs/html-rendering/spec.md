# html-rendering Specification

## Purpose

Defines the single rendering contract shared by initial page responses and live SSE patches, so an HTML component library stays interchangeable.

## Requirements

### Requirement: Accepted HTML inputs
The framework SHALL accept four HTML input forms: a value rendering to a writer with a request context, a value rendering to a writer without a context, a string, and a byte slice. Detection SHALL be structural rather than by concrete type name.

#### Scenario: Renderer with context
- **WHEN** a handler passes a value whose render method takes a request context and a writer
- **THEN** the framework calls it once with the request context and the response writer

#### Scenario: Renderer without context
- **WHEN** a handler passes a value whose render method takes only a writer
- **THEN** the framework calls it once with the response writer and no adapter package

#### Scenario: Unsupported value
- **WHEN** a handler passes a value matching none of the accepted forms
- **THEN** the framework returns an error instead of writing a partial response

#### Scenario: Render error
- **WHEN** a renderer returns an error before any bytes were written
- **THEN** the framework responds `500` and marks the request span as failed

### Requirement: Trusted HTML input
The framework SHALL accept a string or a byte slice as already-rendered HTML and send it unchanged.

#### Scenario: Raw string
- **WHEN** a handler passes the string `<b>x</b>` as HTML
- **THEN** the client receives `<b>x</b>` without escaping

#### Scenario: Trusted input documentation
- **WHEN** a caller passes user-controlled text as a raw string
- **THEN** the result is unescaped by design, so escaping remains the caller's responsibility

### Requirement: Escaping ownership
Escaping SHALL be the responsibility of the renderer, not the framework. The framework SHALL NOT re-escape rendered output.

#### Scenario: Rendered output preserved
- **WHEN** a renderer writes text containing `<` characters already escaped as `&lt;`
- **THEN** the framework writes those bytes unchanged

### Requirement: Same contract for live patches
The HTML sent in a live SSE patch SHALL accept the same four input forms as a full page response.

#### Scenario: Component reused
- **WHEN** the same HTML value is used for a full page response and for an SSE element patch
- **THEN** both produce identical HTML bytes

### Requirement: No HTML library lock-in
The core package SHALL NOT import or depend on a specific HTML component library, and SHALL reach compatible values only through structural interfaces.

#### Scenario: Alternative library
- **WHEN** an application uses another component library whose node type renders to a writer
- **THEN** it works without a core package change and without an adapter package

#### Scenario: Dependency surface
- **WHEN** the core package is inspected for dependencies
- **THEN** no HTML template engine or component library appears among them
