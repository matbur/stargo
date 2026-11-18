# Proposal

## Why

Stargo handlers can read Datastar signals but not ordinary HTML form submissions.
A handler that receives `application/x-www-form-urlencoded` or
`multipart/form-data` must either parse the body itself or give up on uploads, and
every implementation repeats the same limit and cleanup decisions. The reference
implementation provides one bounded reader for both encodings; Stargo needs the
equivalent before cookies, sessions, or authenticated uploads make sense.

## What Changes

- Add a `forms` package with `Read(*http.Request, ...Option) (*Form, error)`.
- Read the query string for `GET` and `HEAD`; read the body for other methods.
- Support `application/x-www-form-urlencoded` and `multipart/form-data`.
- Enforce limits through functional options, with defaults:
  - 10 MiB body, matching the reference default.
  - 1000 text fields.
  - 1 MiB per text field.
  - 10 files.
  - 1 MiB kept in memory per file before spooling to disk.
  - No separate per-file limit below the body limit.
- Expose `Form` with field lookup (`Get`, `Values`), file lookup (`File`, `Files`),
  and `Close` for temporary-file cleanup.
- Expose `UploadFile` with client filename, content type, size, `Open`, and
  `ReadAll`. Filenames are reduced to their base name and remain untrusted.
- Return sentinel errors for malformed, oversized, and unsupported bodies, so a
  handler can map them to `400`, `413`, and `415`.
- Skip file inputs left empty by the browser instead of reporting zero-byte uploads.
- Document the package in the README and remove forms from the not-included list.
- Out of scope: sessions, authentication, field validation, JSON bodies, a query
  string builder, multipart response writing, and a `Ctx` convenience method.

## Capabilities

### New Capabilities

- `form-parsing`: bounded reading of URL-encoded and multipart form submissions,
  including field and file access, limits, and cleanup rules.

### Modified Capabilities

None.

## Impact

- New `forms/` package using only the standard library.
- New tests cover both encodings, limit boundaries, malformed bodies, empty file
  inputs, and temporary-file cleanup.
- README describes the reader and its error mapping.
- No changes to `stargo`, `sse`, `signals`, or the examples.
