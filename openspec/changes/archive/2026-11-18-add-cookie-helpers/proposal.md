# Proposal

## Why

Handlers that need a cookie today call `http.SetCookie` and `r.Cookie` directly. That
works for the happy path, but nothing checks the rules a browser silently enforces:
a `__Host-` cookie without `Secure`, `Path=/`, and no `Domain` is dropped, adding a
cookie after the header is flushed has no effect, and a re-sent `Cookie` header with
the same name resolves to whichever match the stdlib returns. Signed cookies, which
login flows need before sessions exist, have no support at all.

The reference implementation wraps all of this in one small module: validated
`Set-Cookie` writing, prefix rules, request parsing with last-one-wins semantics, and
an HMAC-signed JSON cookie with key rotation. Stargo needs the equivalent before
point 3 of the roadmap and any authenticated example can be built.

## What Changes

- Add a `cookies` package with `Set`, `Delete`, and `Read`.
- `Set` writes one `Set-Cookie` header from an `*http.Cookie`, writing `Path`,
  `Domain`, `Expires`, `Max-Age`, `HttpOnly`, `Secure`, and `SameSite` when set.
- Validate before writing and return `ErrInvalid` instead of letting the stdlib
  silently drop or sanitize a value:
  - names must be RFC 6265 tokens and must not be attribute names;
  - values must be printable ASCII without `;`, `,`, control characters, or the
    quote/backslash pair;
  - `Path` and `Domain` must not contain `;` or `,`;
  - `SameSite` must be `Lax`, `Strict`, `None`, or unset.
- Enforce cookie prefixes: `__Host-` needs `Secure`, `Path=/`, and no `Domain`;
  `__Secure-` needs `Secure`. `SameSite=None` implies `Secure`.
- `Delete` sends a clearing cookie with the same name, path, domain, and flags.
- `Read` parses every `Cookie` header line into a name-to-value map. Later lines win,
  and within a line later pairs win; malformed pairs are skipped.
- Add `NewSigner`, `Signer.Set`, `Signer.Get`, and `Signer.Delete` for a cookie whose
  JSON payload the client can read but not change:
  - HMAC-SHA256 over a versioned, base64url payload carrying data, issue time, and
    expiry;
  - the first key signs, every key verifies, so keys rotate without logging users
    out;
  - the signing key is derived from the configured key and the cookie name, so one
    secret serves several cookies and a value cannot move to another name;
  - keys must be at least 32 bytes, and the encoded value must fit a browser cookie.
- Document the package in the README and remove cookies from the not-included list.
- Out of scope: sessions, authentication, encryption of cookie contents, a cookie
  jar for outbound requests, and `Ctx` convenience methods.

## Capabilities

### New Capabilities

- `cookies`: validated cookie writing, deletion, request parsing with last-one-wins
  semantics, and HMAC-signed JSON cookies with key rotation.

### Modified Capabilities

None.

## Impact

- New `cookies/` package using only the standard library.
- New tests cover attribute rendering, prefix and value validation, deletion,
  multi-line request headers, tampering, expiry, and key rotation.
- README gains a cookies section; sessions stay in the not-included list.
- No changes to `stargo`, `forms`, `signals`, or the examples.
