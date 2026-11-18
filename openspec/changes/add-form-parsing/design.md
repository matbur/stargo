# Design

## Context

See `proposal.md` for motivation. Constraints:

- `signals.Read` establishes the package shape: `Read(*http.Request, ...Option)`
  returning a value and an error, with a default size limit.
- Stargo handlers already receive `*http.Request` through `stargo.Ctx`, so the
  parser does not need a framework handle.
- `net/http` provides URL decoding and multipart parsing, including spooling large
  parts to temporary files.
- The reference implementation defaults to a 10 MiB body, 1000 fields, 1 MiB per
  field, 10 files, and a 1 MiB memory threshold per file.

## Goals / Non-Goals

**Goals:**

- One bounded reader for both standard form encodings.
- Limits enforced before a handler sees unbounded data.
- Temporary files cleaned up without requiring framework integration.
- Errors a handler can map to `400`, `413`, and `415`.

**Non-Goals:**

- Streaming part-by-part iteration for the handler.
- Field ordering guarantees across different field names.
- Multipart responses, sessions, cookies, or validation.
- A `Ctx` convenience method.

## Decisions

### Package `forms` with a `Read` entry point

```go
form, err := forms.Read(c.R, forms.WithMaxBytes(16<<20))
if err != nil {
    switch {
    case errors.Is(err, forms.ErrTooLarge):
        return stargo.NewHTTPError(http.StatusRequestEntityTooLarge, "form too large")
    case errors.Is(err, forms.ErrUnsupportedMediaType):
        return stargo.NewHTTPError(http.StatusUnsupportedMediaType, "expected a form")
    default:
        return stargo.NewHTTPError(http.StatusBadRequest, "malformed form")
    }
}
defer form.Close()
```

The package mirrors `signals.Read` and imports only the standard library.
Alternative: methods on `stargo.Ctx`; rejected because the parser is useful in
tests and plain handlers, and `Ctx` stays thin.

### Method selects the source

`GET` and `HEAD` read `r.URL.Query()`; every other method reads the body and
requires `application/x-www-form-urlencoded` or `multipart/form-data`. This
matches the reference and keeps `method="get"` forms working with the same handler.

### URL-encoded bodies read explicitly

`http.Request.ParseForm` silently caps URL-encoded bodies at 10 MiB and merges
query parameters into the result, so it cannot honor a configurable body limit or
separate body from query. The reader instead:

1. reads at most `maxBytes + 1` bytes from the body,
2. returns `ErrTooLarge` when the extra byte exists,
3. decodes with `url.ParseQuery`, mapping a decode error to `ErrMalformed`.

`url.Values` preserves submission order per key, which satisfies `Values`.

### Multipart bodies use `net/http` multipart support

The reader wraps the body in `http.MaxBytesReader(nil, body, maxBytes)`, parses the
media type, and calls `ParseMultipartForm(memoryBytes)`. That gives battle-tested
boundary handling, per-part headers, and spooling to disk past the memory
threshold. After parsing, the reader enforces field count, field size, file count,
and optional per-file size by inspecting the parsed maps.

Consequence: a single oversized multipart field is buffered before rejection, but
never beyond the body limit. Alternative: a custom streaming parser with early
per-part aborts; rejected for now because it duplicates subtle MIME handling
without changing the observable contract.

`ParseMultipartForm` treats a file part with an empty filename as a text field, so
empty file inputs produce no upload and count toward the field limit. Go's
`multipart` package already reduces filenames to their base name.

### Form and UploadFile shapes

```go
type Form struct{ ... }
func (f *Form) Get(name string) string
func (f *Form) Values(name string) []string
func (f *Form) File(name string) *UploadFile
func (f *Form) Files(name string) []*UploadFile
func (f *Form) Close() error

type UploadFile struct {
    Filename    string
    ContentType string
    Size        int64
}
func (f *UploadFile) Open() (multipart.File, error)
func (f *UploadFile) ReadAll() ([]byte, error)
func (f *UploadFile) Close() error
```

`UploadFile` wraps `*multipart.FileHeader`; `Open` uses `header.Open`, and
`Close` closes whichever file the caller opened. `Form.Close` calls
`MultipartForm.RemoveAll` and is idempotent.

The Go server also removes multipart temporary files when the request ends, so a
handler that forgets `Close` leaks nothing past the response; `Close` releases
them earlier and keeps tests deterministic.

### Limits as options with exported defaults

Options: `WithMaxBytes`, `WithMaxFields`, `WithMaxFieldBytes`, `WithMaxFiles`,
`WithMaxFileBytes`, `WithMemoryBytes`. Defaults live as exported constants so a
handler can derive its own limit. `WithMaxFileBytes(0)` means no separate per-file
limit below the body limit.

Negative limits panic, matching the package style for programmer errors. Values
over the limits return `ErrTooLarge`, not a panic, because they come from the
client.

### Sentinel errors

```go
var (
    ErrMalformed            = errors.New("forms: malformed form body")
    ErrTooLarge             = errors.New("forms: form body too large")
    ErrUnsupportedMediaType = errors.New("forms: expected a form body")
)
```

Errors wrap one sentinel with detail via `fmt.Errorf("%w: ...")`. The package
does not import `stargo` to build HTTP errors: that would tie a pure parser to the
framework and risk a cycle if `stargo.Ctx` later gains a `forms` method. The README
instead shows the mapping from each sentinel to a status code.

## Risks / Trade-offs

- Multipart values are buffered before per-field rejection → body limit bounds the
  allocation; tests cover the boundary.
- `http.MaxBytesReader(nil, ...)` relies on a nil writer being allowed → verified
  by the implementation test that a body over the limit yields `ErrTooLarge`.
- `url.Values` does not preserve order across different field names → the spec
  promises order only per key.
- Temporary files live until `Close` or request end → documented in the package and
  README.

## Migration Plan

Additive package. No callers change. Rollback removes `forms/` and the README
section.

## Open Questions

None.
