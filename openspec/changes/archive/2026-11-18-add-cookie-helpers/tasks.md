# Tasks

## 1. Writing and validation

- [x] 1.1 Create `cookies/` with package docs, `ErrInvalid`, and `Set(http.ResponseWriter, *http.Cookie) error` that renders one `Set-Cookie` header through `http.SetCookie`; verify tests for path, domain, expiry, max age, HTTP-only, secure, and same-site attributes, for several cookies in one response, and for `SameSite=None` adding `Secure`.
- [x] 1.2 Validate names, values, path, domain, and same-site before writing; verify tests for an empty name, spaces, `;`, `,`, `=`, non-ASCII, an attribute name such as `expires`, control characters in the value, `;` and `,` in the path and domain, and a `SameSite` outside the three constants, each returning `ErrInvalid` with no header written.
- [x] 1.3 Enforce the `__Host-` and `__Secure-` prefix rules after the `SameSite=None` promotion; verify tests for a valid host cookie, each missing requirement of `__Host-`, a `__Secure-` cookie without `Secure`, and case-insensitive prefix matching.

## 2. Deletion

- [x] 2.1 Implement `Delete(http.ResponseWriter, *http.Cookie) error` that copies the cookie, clears the value, sets a zero max age and a past expiry, validates, and writes; verify the header carries the supplied path, domain, and flags, that repeated calls write independent headers, that the caller's struct is unchanged, and that an invalid name returns `ErrInvalid`.

## 3. Reading

- [x] 3.1 Implement `Read(*http.Request) map[string]string` that walks every `Cookie` header line, splits pairs at the first `=`, skips empty names, unquotes double-quoted values, and lets later entries win; verify tests for one line with several pairs, two lines defining the same name, one line defining a name twice, a malformed pair alongside valid ones, `token=a=b`, a quoted value, and a request without cookies.

## 4. Signed cookies

- [x] 4.1 Add `NewSigner`, `SignerOption`, and the `WithMaxAge`, `WithPath`, `WithDomain`, `WithSecure`, `WithHTTPOnly`, and `WithSameSite` options with the documented defaults; verify tests for the defaults, each override, an invalid name, no keys, a key shorter than 32 bytes, a non-positive max age, and an invalid prefix configuration, each returning `ErrInvalid`.
- [x] 4.2 Implement `Signer.Set` and `Signer.Get` with the `v1` payload format, per-name key derivation, and constant-time signature checks; verify a round trip preserves the data and exposes issue and expiry times, and that a missing cookie, a changed payload, a changed signature, a wrong version, a non-JSON payload, and a value presented under another cookie name all report no value.
- [x] 4.3 Enforce expiry and rotation in `Get`, and the browser size limit in `Set`; verify a hand-built expired payload reports no value, a signer with a new key first and an old key second accepts values signed with either key and signs new values with the first, and an oversized payload returns an error without writing a header.
- [x] 4.4 Implement `Signer.Delete` using the configured name, path, domain, and flags; verify the clearing header matches a value written by the same signer.

## 5. Documentation

- [x] 5.1 Add a README cookies section showing `cookies.Set`, `cookies.Delete`, `cookies.Read`, and a `NewSigner` round trip, and state that signed payloads are readable and must not hold secrets; remove cookies from the not-included list while keeping sessions there. Verify the snippets compile in a temporary test.

## 6. Integration checks

- [x] 6.1 Run `gofmt -l .`, `go vet ./...`, and `go test ./...` in the root module and in `examples/chat-room`, and confirm all are clean.
- [x] 6.2 Run a real server that sets a plain cookie and a signed cookie, confirm `curl` receives both `Set-Cookie` headers, that a second request sends them back and the signed payload verifies, and that deleting both clears them.

## Workflow follow-up

- Archive the change with `/opsx-archive` after review and confirm the new `cookies` spec exists in `openspec/specs/`.
