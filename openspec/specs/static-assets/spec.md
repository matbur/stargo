# static-assets Specification

## Purpose

Serves application static files from an embedded filesystem or a directory with content hashing, cache policy, precompressed variants, and path safety.

## Requirements

### Requirement: Static sources and allowlist
The framework SHALL serve files from an `fs.FS` or a directory, indexing only regular files whose extension is in an explicit content-type allowlist, and SHALL return not found for everything else.

#### Scenario: Embedded file
- **WHEN** an embedded filesystem contains `app.css` and the handler receives a request for it
- **THEN** the response body is the file and the content type is `text/css`

#### Scenario: Directory file
- **WHEN** the handler is built from a directory containing `app.js`
- **THEN** the response body is the file and the content type is `text/javascript`

#### Scenario: Unknown extension
- **WHEN** the source contains a file with an extension outside the allowlist
- **THEN** a request for it returns 404 and the file body is not served

#### Scenario: Directory listing
- **WHEN** a request targets a directory path
- **THEN** the handler returns 404 and never lists the directory

### Requirement: Hashed and stable URLs
The handler SHALL hash every indexed file at construction, expose a hashed URL with a content hash before the extension and a stable URL with the plain name, and apply the configured mount prefix to both.

#### Scenario: Hashed URL
- **WHEN** `URL("app.css")` is called for an indexed file
- **THEN** the result contains `/app.<hash>.css` under the configured prefix and serves that file

#### Scenario: Stable URL
- **WHEN** `StableURL("app.css")` is called
- **THEN** the result is the prefixed plain path and serves the same file

#### Scenario: Unknown name
- **WHEN** a URL helper is called for a name that was not indexed
- **THEN** it panics because asset names are programmer-authored constants

#### Scenario: Wrong hash
- **WHEN** a request uses a hash that does not match the indexed file
- **THEN** the handler returns 404

### Requirement: Static cache policy
Hashed paths SHALL be served as immutable for one year, stable paths SHALL require revalidation, and both SHALL carry a strong ETag derived from the content.

#### Scenario: Immutable hashed
- **WHEN** a hashed path is served
- **THEN** the response carries `Cache-Control: public, max-age=31536000, immutable`

#### Scenario: Revalidated stable
- **WHEN** a stable path is served
- **THEN** the response carries `Cache-Control: no-cache` and an ETag

#### Scenario: Not modified
- **WHEN** a request repeats the current ETag in `If-None-Match`
- **THEN** the response is 304 with no body

### Requirement: Precompressed variants
The handler SHALL serve an existing `.br`, `.zst`, or `.gz` sibling when the request accepts that encoding, choosing by quality and preferring brotli, then zstd, then gzip, and SHALL fall back to the identity file.

#### Scenario: Brotli preferred
- **WHEN** a file has brotli and gzip siblings and the request accepts both with equal quality
- **THEN** the brotli variant is served with `Content-Encoding: br`

#### Scenario: Quality wins
- **WHEN** the request gives gzip a higher quality than brotli
- **THEN** the gzip variant is served

#### Scenario: No accepted variant
- **WHEN** the request accepts no encoding that exists for the file
- **THEN** the identity file is served without a content encoding

#### Scenario: Cache separation
- **WHEN** a file has precompressed variants
- **THEN** responses carry `Vary: Accept-Encoding`

### Requirement: Conditional and range requests
Static responses SHALL support `HEAD`, byte ranges, and validators without reimplementing HTTP file serving.

#### Scenario: Range request
- **WHEN** a request asks for bytes 0 through 3 of a file
- **THEN** the response is 206 with exactly those bytes

#### Scenario: Head request
- **WHEN** a `HEAD` request targets a file
- **THEN** the headers match a `GET` response and no body is written

### Requirement: Static path safety
The handler SHALL reject absolute paths, parent segments, backslashes, NUL bytes, and hidden path segments, and SHALL NOT follow symlinks inside a directory source.

#### Scenario: Parent traversal
- **WHEN** a request path contains a `..` segment
- **THEN** the response is 404

#### Scenario: Hidden file
- **WHEN** the source contains a hidden file or the request targets a hidden segment
- **THEN** the file is not indexed and the response is 404

#### Scenario: Symlink escape
- **WHEN** a directory source contains a symlink to a file outside the root
- **THEN** the link is not indexed and the response is 404

#### Scenario: Unsupported method
- **WHEN** a request uses a method other than `GET` or `HEAD`
- **THEN** the response is 405 with an `Allow` header
