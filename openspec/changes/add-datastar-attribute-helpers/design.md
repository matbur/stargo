# Design

## Context

See `proposal.md` for motivation. Constraints that shape the approach:

- `gomponents` writes attribute names verbatim and escapes only values, so a helper must
  validate names itself.
- `openspec/specs/html-rendering/spec.md` forbids the core package from depending on a
  specific HTML library. The helper therefore renders to an `io.Writer` instead of
  returning `gomponents` nodes.
- `sse/options.go` establishes the functional-option style, and `route/route.go`
  establishes panic on invalid construction.
- Attribute HTML output must stay byte-identical so existing view and handler tests keep
  passing. Action paths are normalized to single quotes, so chat-room assertions that
  expect double quotes change.
- Datastar action paths are JavaScript string literals embedded in an expression, while
  payload values and signal filters are raw JavaScript.

## Goals / Non-Goals

**Goals:**

- One typed entry point for Datastar event attributes with validated names and modifiers.
- One typed entry point for fetch actions with safe path quoting and typed options.
- Stable, canonical output independent of option order.
- Prefix support without documenting or encouraging it in the first release.
- No changes to the rendering contract or `sse`.

**Non-Goals:**

- Other Datastar attributes (`init`, `signals`, `bind`, `attr`, `json-signals`).
- Pro actions (`clipboard`, `intl`, and similar).
- Query parameter maps on actions; route URL builders already produce complete URLs.
- Loading, pinning, serving, or embedding the Datastar JavaScript bundle.
- Automatic conversion of camelCase events; the helper rejects them instead.

## Decisions

### Package and value shape

`datastar` exposes an `Attributes` value with an unexported prefix. `Default()` returns
the `data-` instance, `New(prefix)` returns an aliased one, and methods render
attributes. A package-level `On` delegates to `Default()` so the common call site stays
short. Files import the package as `ds`, matching the examples and documentation:

```go
import ds "github.com/matbur/stargo/datastar"

ds.On("click", expr)
ds.New("data-star-").On("click", expr)
```

The package keeps the name `datastar`; `ds` is the documented import alias rather than a
package name. Go import aliases are per file, so nothing enforces the convention, but
every Stargo example and snippet uses it consistently.

Alternatives: package-level functions only (prefix becomes a per-call option or a mutable
global) and a builder (extra type, partial state). Both were rejected in exploration.

### Functional options

`OnOption` follows `sse.Option`: an unexported config struct plus exported constructors
such as `WithPrevent()` and `WithDebounce(time.Duration)`. Options only set fields; the
render step validates the final combination. This keeps duplicate detection and ordering
in one place.

Alternatives: an options struct (rigid, ambiguous zero values) and a builder (mutable,
more API surface).

### Own renderer instead of `gomponents.Node`

The helper returns a small struct with `Render(io.Writer) error`. `gomponents` accepts it
structurally, as does any renderer using the same contract, and the package avoids an
import edge to the HTML library.

Consequence: attribute-value escaping must match HTML escaping. The implementation uses
the same six replacements as `gomponents` (NUL, `"`, `'`, `&`, `<`, `>`). A test renders
a corpus through both and compares bytes, so divergence fails CI. Tests may import
`gomponents`; production code may not.

Alternative: import `gomponents` and return `g.Node`. Rejected because it ties the helper
to one library and adds a dependency edge the core deliberately avoids.

### Fixed modifier order

Modifiers render in this order, matching Stario and the Datastar reference reading order:

```
once, passive, capture, window|document|outside, prevent, stop,
delay, debounce, throttle, viewtransition, case
```

Duplicate modifiers collapse. The order is fixed after option application, not by call
order, so output is stable and testable.

### Action builders return expression strings

Fetch actions are values inside attributes, not attributes themselves, so `Get`, `Post`,
`Put`, `Patch`, and `Delete` return a string. It composes with `On` and with hand-written
JavaScript:

```go
ds.On("click", ds.Post(userPath))

submit := "if ($message.trim()) { " + ds.Post(sendPath) + "; $message = ''; }"
```

The verb is an unexported parameter of one builder, so all verbs share quoting, option
rendering, and validation. A separate action type was rejected: it would need conversion
to a string at every use site and would not make composition safer.

### Path quoting

Paths are encoded as single-quoted JavaScript literals. The encoder escapes `'`, `\`,
newline, carriage return, U+2028, and U+2029, and rejects NUL. This is the boundary that
prevents a path from ending the literal and injecting JavaScript; tests use quote,
backslash, and line-separator cases.

Single quotes match Stario and Datastar's reference examples. The alternative, JSON-style
double quotes, would preserve the chat-room's current bytes but is noisier in HTML because
every quote becomes `&#34;`. One canonical form is worth the test updates.

### Action options

`ActionOption` uses the same functional style as `OnOption`, with an unexported
`actionConfig`. Options render in this fixed order:

```
contentType, filterSignals, selector, headers, openWhenHidden,
payload, retry, retryInterval, retryScaler, retryMaxWait,
retryMaxCount, requestCancellation
```

Rendering rules:

- `contentType` is `json` or `form`; `selector` requires `form` and panics otherwise.
- Headers are quoted string literals with HTTP field-name validation.
- Payload keys are validated; payload values are raw JavaScript expressions and are
  documented as trusted input.
- Signal filters are regex literals; patterns reject empty input, unescaped `/`, and line
  breaks.
- Durations render as integer milliseconds; retry mode, scaler, wait, and count are
  validated before rendering.
- `requestCancellation` accepts `auto`, `cleanup`, or `disabled`.

An options struct was rejected for the same reasons as with `On`: optional keys need
presence detection, and zero values are ambiguous.

### Panic on programmer error

Invalid event names, prefixes, paths, option values, and contradictory combinations
(`passive` with `prevent`, selector without the form content type) panic with a
`datastar:` prefix. These cannot come from request data, and `route.New` already treats
the same class of mistake as a panic. The request panic recovery turns it into a `500`,
never a partial page.

### Event and prefix grammar

Events: lowercase ASCII letters, digits, and hyphens; must start with a letter and must
not end with a hyphen. Uppercase and underscore are rejected with a message pointing at
kebab-case. Datastar lowercases attribute names in the browser, so accepting `myEvent`
would silently change its meaning.

Prefix: must match `data-<alias>-`, where `<alias>` is lowercase kebab-case; a missing
trailing hyphen is added. This is the only form Datastar's bundler supports for aliases.
`data-` remains the documented default; `data-star-` works but is not promoted because the
application must also load a matching bundle.

### Example migration scope

Only `data-on*` attributes and fetch action strings move to the helpers. `data-init`,
`data-signals`, `data-bind`, `data-attr`, and `data-msg-id` stay as raw attributes until
their own change.

Chat-room's room view drops its local `jsonString` helper because every use was an action
path. The lobby keeps it for the delete confirmation text, which is not an action. Action
output switches to single quotes, so assertions in the lobby and room view tests change
from `&#34;` to `&#39;`; rendered behavior is otherwise identical.

## Risks / Trade-offs

- **Escaping divergence** -> parity test against `gomponents` output for every escaped
  character, plus a property-style corpus.
- **Action quoting gap** -> a dedicated table test covers every escaped character and
  every line separator in a path.
- **Payload injection** -> payload values are raw JavaScript by design; documentation and
  the package doc state that only trusted expressions may be used.
- **Silent prefix mismatch** -> documented rule that the same prefix must be used by the
  page, every SSE patch, and any raw HTML; mismatch is not detectable at runtime.
- **Name validation gaps** -> tests cover uppercase, whitespace, quotes, `>`, `/`, `=`,
  leading and trailing hyphens, and Unicode.
- **Option API growth** -> keep options minimal to what examples and Datastar's common
  reference need; adding one later is backward compatible.
- **Duplicated escaping code** -> small, pinned by byte comparison with the library the
  examples use.

## Migration Plan

1. Add the `datastar` package, its attribute helpers, and its action helpers with tests.
2. Replace `g.Attr("data-on...")` calls and hand-built action strings in `examples/hello`
   and `examples/chat-room`.
3. Update chat-room assertions to the canonical single-quoted action form.
4. Run the full test suite and confirm attribute names and page behavior are unchanged.
5. Add a short README subsection showing `On` with modifiers and an action.

Rollback is a revert: no persisted state, no protocol change, and raw attribute strings
remain valid.

## Open Questions

None. Prefix documentation visibility and event-case behavior were decided during
exploration; all remaining work is covered by the specs and tasks.
