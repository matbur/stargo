# Tasks

## 1. Client foundation

- [x] 1.1 Create `testing/doc.go` and `Client` with `NewClient`, `URL`, `Close`, cleanup registration, and `WithDefaultHeader`; verify a request target is absolute and that the server closes at cleanup.
- [x] 1.2 Add the cookie jar and `WithoutCookies`, plus `Cookie(name)`; verify a `Set-Cookie` handler round-trips through a later request, the disabled jar stores nothing, and lookup reports presence.
- [x] 1.3 Add `WithoutRedirects` and `WithMaxRedirects`; verify the default follows, the stop option returns the 3xx response unchanged, and an exceeded limit fails the test.

## 2. Requests and encodings

- [x] 2.1 Add `Request`, `Do`, and `Get` with `WithHeader` and `WithBody`; verify a handler sees the method, headers, body, and media type, and a transport failure reports the target.
- [x] 2.2 Add `WithForm` and `PostForm`; verify repeated values survive, the content type is `application/x-www-form-urlencoded`, and `forms.Read` returns the values.
- [x] 2.3 Add `Multipart`, `File`, `WithMultipart`, and `PostMultipart`; verify fields, one file name and content type, the default octet-stream type, repeated files in order, and a boundary that matches the body.

## 3. Responses and assertions

- [x] 3.1 Add `Response` with status, headers, body, request, `Text`, and `DecodeJSON`; verify a text body, a decoded JSON object, and a failure on invalid JSON.
- [x] 3.2 Add `AssertStatus`, `AssertHeader`, `AssertContains`, `AssertNotContains`, and `AssertCount` with request context, truncated bodies, and chainable returns; verify passing calls and record failure messages with a fake `TB`.
- [x] 3.3 Add `Render`; verify a writer renderer, a string, and bytes against the response renderer output.

## 4. SSE events

- [x] 4.1 Add `Stream`, `Open`, status and header accessors, `Next`, and `Close`; verify headers are readable before the first event and that closing stops reads.
- [x] 4.2 Complete the event parser with type, id, retry duration, joined data lines, ordered comments, CRLF handling, several events in order, a payload longer than 64 KiB, and end-of-stream reporting.

## 5. Patches

- [x] 5.1 Add `PatchKind`, `Patch`, `Event.Patch`, and `ErrNotPatch`; verify element mode, selector, namespace, view transition values, joined multi-line elements, decoded signals, the only-if-missing flag, and non-patch events.
- [x] 5.2 Add `Stream.NextPatch`; verify a ping is skipped, the next patch is returned, and an invalid signal payload fails with the raw event.

## 6. Integration and documentation

- [x] 6.1 Add an end-to-end test against a real `stargo.App` that reads a form and a cookie, sets a cookie, and serves element and signal patches; verify the client sees the form values, the cookie round-trips, and both patches parse.
- [x] 6.2 Add a README testing section and finish the package docs with the import path, failure behavior, and the no-DOM limitation; verify the snippets compile in a temporary test.
- [x] 6.3 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`, and confirm all are clean.
- [x] 6.4 Run `openspec validate --changes --strict` and confirm the change and its delta spec pass.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm the new `testing-helpers` spec contains the requirements.
