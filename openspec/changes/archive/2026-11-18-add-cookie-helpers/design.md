# Design

## Context

See `proposal.md` for motivation. Constraints:

- Stargo handlers receive `*http.Request` and `http.ResponseWriter` through
  `stargo.Ctx`, so cookie helpers work on the standard library types.
- `forms.Read` established the convention: a small package that returns values and
  sentinel errors, usable from plain handlers and tests.
- `net/http` already renders `Set-Cookie` and parses `Cookie` headers, but it
  sanitizes invalid input silently, keeps only the first match per name, and has no
  signing support.
- The reference implementation defaults a signed cookie to a 14-day max age, path
  `/`, `Secure`, `HTTPOnly`, and `SameSite=Lax`, and requires 32-byte keys.

## Goals / Non-Goals

**Goals:**

- Validated writing with errors instead of silent sanitizing.
- Prefix rules enforced before a browser drops the cookie.
- One deletion helper that matches how the cookie was set.
- Request parsing that matches browser behavior when a name repeats.
- Signed cookies with key rotation, usable before sessions exist.

**Non-Goals:**

- Sessions, authentication, or server-side cookie storage.
- Encryption; the payload is readable by the client by design.
- A cookie jar for outbound HTTP requests.
- `Ctx` convenience methods.

## Decisions

### Package `cookies` over `*http.Cookie`

```go
func Set(w http.ResponseWriter, cookie *http.Cookie) error
func Delete(w http.ResponseWriter, cookie *http.Cookie) error
func Read(request *http.Request) map[string]string

type Signer struct{ ... }
func NewSigner(name string, keys []string, opts ...SignerOption) (*Signer, error)
func (s *Signer) Set(w http.ResponseWriter, data map[string]any) error
func (s *Signer) Get(request *http.Request) (SignedValue, bool)
func (s *Signer) Delete(w http.ResponseWriter) error
```

`http.Cookie` already carries every attribute the standard defines, and
`http.SetCookie` writes one header per call without touching earlier ones.
Alternative: a Stargo-owned `Cookie` struct; rejected because it duplicates the
stdlib and forces conversions at every call site.

### Validation before rendering

`Set` validates, then calls `http.SetCookie`. It returns `ErrInvalid`, wrapped with
detail, instead of letting the stdlib strip a bad name or quote a bad value:

- names are RFC 9110 tokens (`A-Z`, `a-z`, `0-9`, `!#$%&'*+-.^_`|~`) and may not be
  attribute names (`expires`, `path`, `domain`, `max-age`, `secure`, `httponly`,
  `samesite`, `partitioned`, and the legacy `comment` and `version`);
- values are printable ASCII without `;`, `,`, `"`, `\`, and control characters;
- `Path` and `Domain` may not contain `;` or `,`;
- an empty `SameSite` field is allowed, any other value must be one of the three
  constants.

`SameSite=None` sets `Secure` on the cookie before rendering, because browsers reject
a `None` cookie without it. This mutates the caller's struct and is documented.

### Prefix rules

`__Host-` requires `Secure`, `Path=/`, and no `Domain`; `__Secure-` requires
`Secure`. The check runs after the `SameSite=None` promotion. Names are compared
case-insensitively, matching browsers and the reference implementation.

### Deletion

`Delete` takes a cookie describing the name and the attributes used when setting it.
It copies the struct, clears `Value`, sets `MaxAge=-1` (rendered as `Max-Age=0`) and
`Expires` to a past time, validates, and writes. Copying avoids mutating the caller's
struct.

### Reading request cookies

`http.Request.Cookie` returns the first match, so a re-sent header would keep the
older value. `Read` instead walks every `Cookie` header line in order, splits on `;`,
splits each pair at the first `=`, skips pairs with an empty name, and assigns into
one map, so the last value wins:

- later header lines win over earlier ones;
- within a line, later pairs win;
- `token=a=b` yields `a=b`;
- a value wrapped in double quotes is unquoted when it is a valid Go string
  literal; otherwise it is kept raw;
- malformed pairs are skipped, never fatal.

The result is a fresh map per call; handlers call it once per request.

### Signed payload format

```
v1.<base64url(json)>.<base64url(hmac)>
```

JSON is `{"d": <data>, "i": <issued unix>, "e": <expiry unix>}`. The signature is
HMAC-SHA256 over `v1.<base64url(json)>` with the derived key, base64url without
padding. `Get` rejects a value unless the version matches, the ASCII check passes,
one of the keys verifies, the JSON decodes into the expected shape, and the expiry is
in the future.

The payload is not encrypted: the client can read it. Store an id, never a secret.

### Key derivation and rotation

Each configured key is turned into a signing key with
`HMAC-SHA256(key, "stargo.cookie." + name)`. One secret can therefore serve several
cookie names, and a value issued for one name cannot be replayed under another.

`keys[0]` signs; every key verifies. Rotation means putting the new key first and
keeping the old one for at least one max-age. `Get` compares signatures with
`hmac.Equal`.

### Signer configuration

Options: `WithMaxAge`, `WithPath`, `WithDomain`, `WithSecure`, `WithHTTPOnly`,
`WithSameSite`. Defaults mirror the reference: 14 days, `/`, no domain, secure,
HTTP-only, `Lax`.

`NewSigner` returns `ErrInvalid` when the name is not a token, no key is given, a key
is shorter than 32 bytes, the max age is not positive, or the prefix rules fail.

`Set` rejects a cookie whose rendered line exceeds 4096 bytes, the smallest size
browsers are required to support, before writing anything. The check uses
`http.Cookie.String()`, so it accounts for the name, value, and attributes together.

### Testing

`httptest.ResponseRecorder` inspects `Set-Cookie` headers. Requests with repeated
`Cookie` lines use `Header.Add`. Expiry and rotation are tested with hand-built
payloads through unexported helpers, so the tests stay deterministic without a
clock abstraction in the public API.

## Risks / Trade-offs

- Mutating `cookie.Secure` for `SameSite=None` surprises callers → documented on
  `Set`.
- `Read` keeps only the last value per name, so a handler cannot see all values →
  browsers send one value per name; last-wins matches the reference.
- Rejecting quotes and backslashes in values is stricter than RFC 6265 allows →
  those values are rare, and the alternative is silent misquoting.
- The signer checks expiry after verifying the signature, so an attacker cannot
  learn anything from the timing of a forged expiry.

## Migration Plan

Additive package. No callers change. Rollback removes `cookies/` and the README
section. Sessions remain a separate change.

## Open Questions

None.
