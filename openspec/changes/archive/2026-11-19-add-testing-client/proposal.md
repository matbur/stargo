# Proposal

## Why

Every Stargo test builds its own transport plumbing: `httptest` recorders or
servers, hand-assembled form bodies, ad-hoc cookie handling, and `strings.Contains`
checks. There is no shared way to exercise a real app with cookies and redirects,
encode a multipart upload, or read Datastar SSE patches. Stario has a testing
package for exactly this; Stargo has the primitives (`forms`, `cookies`, `sse`,
`signals`) but no glue, so each test repeats the same setup and the example tests
drift.

## What Changes

- Add a test client package that starts an `httptest` server for any
  `http.Handler`, including `*stargo.App`, and cleans it up when the test ends.
- Add cookie jar support by default, plus options to disable cookies, stop
  following redirects, cap redirects, and set default headers.
- Add request helpers for buffered requests, URL-encoded forms, and multipart
  bodies with fields and files. Buffered responses expose status, headers, body,
  text, cookie lookup, and JSON decoding.
- Add chainable assertions for status, headers, body fragments, and fragment
  counts, reporting failures through a small `TB` interface with the request
  and response context.
- Add a rendered-HTML helper that renders a page value with the same renderer
  used by responses and patches.
- Add SSE helpers that open a streaming request, parse events with type, id,
  retry, data, and comments, and parse Datastar element and signal patches into
  typed values.
- Add integration tests that use the client with `forms`, `cookies`, and a real
  `stargo.App` SSE handler.
- Document the package in the README and package docs.
- Out of scope: virtual time, tracer assertions, compressed response decoding,
  and a browser-like DOM or CSS selector engine.

## Capabilities

### New Capabilities

- `testing-helpers`: in-process HTTP test client, request encoding, response
  assertions, and SSE event and patch parsing for tests.

### Modified Capabilities

None.

## Impact

- New package at `testing/` (package `stargotest`) with a client, response,
  multipart encoding, assertions, and an SSE parser.
- New tests under `testing/` covering the client against a real `stargo.App`,
  form and multipart handlers, cookie and redirect behavior, SSE patches, and
  assertion failures.
- README gains a testing section; no changes to `forms`, `cookies`, `sse`, or
  `signals`.
- No new module dependency.
