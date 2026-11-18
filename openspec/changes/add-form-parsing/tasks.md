# Tasks

## 1. Package skeleton and options

- [x] 1.1 Create `forms/` with package docs, a `Read(*http.Request, ...Option) (*Form, error)` signature, `Option`/`config`, exported defaults for body, field count, field size, file count, and memory thresholds, and the `WithMaxBytes`, `WithMaxFields`, `WithMaxFieldBytes`, `WithMaxFiles`, `WithMaxFileBytes`, and `WithMemoryBytes` options; verify a table test that default values match the documented limits and that negative options panic.
- [x] 1.2 Add the `ErrMalformed`, `ErrTooLarge`, and `ErrUnsupportedMediaType` sentinels and a test that every wrapped error satisfies `errors.Is` for its sentinel.

## 2. Source selection and URL-encoded bodies

- [x] 2.1 Implement method dispatch so `GET` and `HEAD` read the query string and all other methods read the body; verify tests for a query-string form, a body form, and a missing body returning an empty form.
- [x] 2.2 Implement bounded URL-encoded reading: read at most `maxBytes+1` bytes, return `ErrTooLarge` when the extra byte is present, and decode with `url.ParseQuery`; verify tests for percent escapes, `+` as space, repeated keys preserving submission order, a malformed escape returning `ErrMalformed`, and a body exactly at and one byte over the limit.
- [x] 2.3 Reject unsupported media types, including a URL-encoded body declaring a non-UTF-8 charset; verify tests for `application/json`, a missing `Content-Type`, `text/plain`, and `charset=iso-8859-1`.

## 3. Multipart bodies

- [x] 3.1 Parse `multipart/form-data` with `http.MaxBytesReader(nil, body, maxBytes)` and `ParseMultipartForm(memoryBytes)`; verify a test that a text part and a file part produce the expected field value, filename, content type, and size, and that an oversized body yields `ErrTooLarge`.
- [x] 3.2 Handle edge cases: empty filename reports no file, a client path in the filename keeps only the base name, and a missing or malformed boundary returns `ErrUnsupportedMediaType` or `ErrMalformed`; verify one test per case.
- [x] 3.3 Enforce field count, field size, file count, and the optional per-file limit after parsing, returning `ErrTooLarge`; verify boundary tests at the limit and one over for each limit, including `WithMaxFileBytes(0)` meaning no per-file limit.

## 4. Form and UploadFile values

- [x] 4.1 Implement `Form.Get`, `Form.Values`, `Form.File`, and `Form.Files` with submission order per field name; verify tests for the first value, all values, a missing field returning `""` and no values, and a missing file returning nil.
- [x] 4.2 Implement `UploadFile` with `Filename`, `ContentType`, `Size`, `Open`, `ReadAll`, and `Close` over `*multipart.FileHeader`; verify that `ReadAll` returns the uploaded bytes and that `Open` followed by `Close` works.
- [x] 4.3 Implement `Form.Close` as an idempotent `MultipartForm.RemoveAll` with a nil-safe receiver; verify that closing a form with a file spooled to disk removes the temporary file, that closing twice returns no error, and that a small upload below the memory threshold creates no temporary file.

## 5. Documentation

- [x] 5.1 Add a README forms section showing `forms.Read`, `defer form.Close()`, field and file access, and mapping the three sentinels to `400`, `413`, and `415`; remove forms from the not-included list. Verify the snippets compile in a temporary test.

## 6. Integration checks

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`, and confirm all are clean.
- [x] 6.2 Send a real URL-encoded and a real multipart request to a running handler and confirm field and file values arrive intact and oversized bodies fail with the expected error.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm the new `form-parsing` spec exists in `openspec/specs/`.
