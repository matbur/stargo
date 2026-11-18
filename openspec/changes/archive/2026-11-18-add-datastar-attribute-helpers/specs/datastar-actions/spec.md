# Spec Delta

## Purpose

Defines how Stargo builds Datastar fetch action strings from typed inputs, including path quoting, typed fetch options, canonical option order, and validation.

## ADDED Requirements

### Requirement: Action rendering
A helper SHALL render a fetch action for the verbs `get`, `post`, `put`, `patch`, and `delete` as `@<verb>('<path>')`.

#### Scenario: Simple post
- **WHEN** a caller builds a POST action for `/command`
- **THEN** the rendered string is `@post('/command')`

#### Scenario: Each verb
- **WHEN** a caller builds actions for all five verbs
- **THEN** each rendered string starts with its verb and carries the quoted path

### Requirement: Path quoting
The path SHALL be encoded as a single-quoted JavaScript string literal, so the path cannot terminate the string or add JavaScript.

#### Scenario: Quote in path
- **WHEN** the path contains `'`
- **THEN** the rendered literal escapes it as `\'`

#### Scenario: Backslash in path
- **WHEN** the path contains `\`
- **THEN** the rendered literal escapes it as `\\`

#### Scenario: Line separators
- **WHEN** the path contains a newline, carriage return, U+2028, or U+2029
- **THEN** the rendered literal uses a JavaScript escape sequence for it

#### Scenario: NUL in path
- **WHEN** the path contains NUL
- **THEN** construction panics instead of rendering

### Requirement: Option object rendering
Supplied fetch options SHALL render inside one `{...}` object after the path, separated by `, `, and options SHALL appear in a fixed order independent of their call order.

#### Scenario: No options
- **WHEN** no options are supplied
- **THEN** the action renders only the verb and quoted path, with no object

#### Scenario: Several options
- **WHEN** several options are supplied
- **THEN** they render as `name: value` pairs in the defined order

#### Scenario: Order independence
- **WHEN** the same options are supplied in two different orders
- **THEN** both actions render byte-identical strings

### Requirement: Content type and selector
The content type SHALL be `json` or `form`, and a selector SHALL be accepted only together with the form content type.

#### Scenario: Form content type
- **WHEN** a caller selects the form content type
- **THEN** the action contains `contentType: 'form'`

#### Scenario: Selector with form
- **WHEN** a caller selects the form content type and a selector
- **THEN** the action contains both `contentType: 'form'` and the quoted selector

#### Scenario: Selector without form
- **WHEN** a caller supplies a selector without the form content type
- **THEN** construction panics with a message naming the dependency

#### Scenario: Unknown content type
- **WHEN** a caller supplies a content type other than `json` or `form`
- **THEN** construction panics

### Requirement: Headers option
Headers SHALL render as a JavaScript object whose keys and values are quoted string literals.

#### Scenario: Custom headers
- **WHEN** a caller supplies header `X-Trace` with value `abc`
- **THEN** the action contains `headers: {'X-Trace': 'abc'}`

#### Scenario: Invalid header name
- **WHEN** a header name is empty or contains characters invalid in an HTTP field name
- **THEN** construction panics

### Requirement: Payload option
The payload SHALL render as a JavaScript object mapping signal names to expressions supplied by the caller.

#### Scenario: Signal expression payload
- **WHEN** a caller supplies payload entry `q` with expression `$query`
- **THEN** the action contains `payload: {q: $query}`

#### Scenario: Payload values are trusted JavaScript
- **WHEN** a payload value contains JavaScript syntax
- **THEN** the helper writes it unchanged and does not quote it as a string

#### Scenario: Invalid payload key
- **WHEN** a payload key is empty or contains whitespace, quotes, or braces
- **THEN** construction panics

### Requirement: Signal filters
Signal filters SHALL render as `filterSignals: {include: /pattern/, exclude: /pattern/}` with each supplied pattern emitted as a JavaScript regular expression literal.

#### Scenario: Include only
- **WHEN** a caller supplies only an include pattern
- **THEN** the action contains `filterSignals: {include: /pattern/}` and no exclude key

#### Scenario: Include and exclude
- **WHEN** a caller supplies both patterns
- **THEN** both appear under `filterSignals`

#### Scenario: Invalid pattern
- **WHEN** a pattern is empty, contains an unescaped `/`, or contains a line break
- **THEN** construction panics

### Requirement: Retry options
Retry options SHALL render the retry mode, interval, scaler, maximum wait, and maximum count with Datastar's option names.

#### Scenario: Retry mode
- **WHEN** a caller selects the `always` retry mode
- **THEN** the action contains `retry: 'always'`

#### Scenario: Retry timings
- **WHEN** a caller supplies a retry interval and maximum wait as durations
- **THEN** the action contains those values as integer milliseconds

#### Scenario: Invalid retry values
- **WHEN** the mode is unknown, a duration is not positive, the scaler is not a finite positive number, or the count is negative
- **THEN** construction panics

### Requirement: Visibility and cancellation options
The helper SHALL render whether the request stays open on a hidden page and which request-cancellation mode applies.

#### Scenario: Open when hidden
- **WHEN** a caller requests the request to stay open on a hidden page
- **THEN** the action contains `openWhenHidden: true`

#### Scenario: Cancellation modes
- **WHEN** a caller selects `auto`, `cleanup`, or `disabled`
- **THEN** the action contains `requestCancellation: '<mode>'`

#### Scenario: Unknown cancellation mode
- **WHEN** a caller supplies a mode outside the supported set
- **THEN** construction panics

### Requirement: Composition with event attributes
An action string SHALL be usable as the expression of an event attribute, and the attribute helper SHALL HTML-escape it like any other expression.

#### Scenario: Action on click
- **WHEN** a caller combines a click event with a POST action
- **THEN** the rendered attribute is `data-on:click="@post(&#39;/command&#39;)"`

#### Scenario: Page and patch parity
- **WHEN** the same action attribute is rendered in a page response and in an SSE element patch
- **THEN** both produce identical bytes
