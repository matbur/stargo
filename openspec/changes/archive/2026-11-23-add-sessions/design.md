# Design

## Identity

A session id is 32 random bytes from `crypto/rand`, encoded with URL-safe
base64. It is opaque, unguessable, and stored in a cookie. The id is not signed:
the store only accepts ids it issued, so a forged id simply misses. The cookie
is a session cookie with no `Max-Age`; the browser drops it when it closes,
while the server enforces both idle and absolute expiry.

## Store contract

`Store` has three methods and operates on a `Record`:

```go
type Record struct {
	Values  map[string]any
	Created time.Time
	Expires time.Time
}

type Store interface {
	Load(ctx context.Context, id string) (Record, bool, error)
	Save(ctx context.Context, id string, record Record) error
	Delete(ctx context.Context, id string) error
}
```

`Created` anchors the absolute lifetime across saves; `Expires` is the earliest
of the idle deadline and the absolute deadline. A durable store serializes
`Values`, so applications should store JSON-friendly values. The memory store
keeps the record in a map and copies it on the way in and out; values
themselves are not deep-copied.

`MemoryStore` expires records lazily on `Load` and `Save`, and exposes `Sweep`
for periodic cleanup. It starts no goroutine, so a process that never loads a
session does not pay for a janitor.

## Manager and middleware

`Manager` owns the cookie attributes, the timeouts, and the store. `New` returns
an error for invalid cookie options, because the `__Host-` prefix rules are
checked by `cookies` at construction.

Middleware reads the cookie, loads the record, and puts a `*Session` in the
request context. A missing or expired record yields a fresh in-memory session
that is not persisted until the handler touches it, so anonymous requests and
static assets do not create store entries or `Set-Cookie` headers.

The session writes its cookie when it is first modified, not when it is loaded.
Handlers must therefore create, rotate, or destroy a session before the
response starts. A write after the response started cannot change headers;
the data is still saved under the existing id, which is documented.

After the handler returns, middleware saves a used session, deletes ids
replaced by `Rotate`, and deletes the record on `Destroy`. Save failures happen
after the response started, so they go to an optional `OnError` callback
instead of changing the status. A load failure happens before the handler and
returns 500.

## Expiration

Loading a session counts as use. Saving recomputes `Expires` as
`min(now + IdleTimeout, Created + AbsoluteTimeout)`, so active sessions slide
forward until the absolute deadline. Defaults are 30 minutes idle and 24 hours
absolute. A zero absolute timeout disables the absolute limit; a non-positive
idle timeout is invalid.

## Cookie defaults

The default name is `__Host-session`, which requires `Secure`, `Path=/`, and no
`Domain`. `Insecure` turns `Secure` off and, when the name is still the default,
renames the cookie to `session` so local HTTP development passes validation.
`SameSite=Lax` covers ordinary navigation while blocking cross-site form posts,
which is the right default before CSRF middleware exists.

## Non-goals

- No authentication, password hashing, login flow, or `RequireUser`
  middleware; those need an account model and form a later change.
- No CSRF token; `SameSite=Lax` is the only current mitigation for cross-site
  mutations.
- No durable store in this change. The contract is deliberately small and is
  documented with the serialization requirement.
- No encrypted or signed id. The id is random, and signing would not add
  protection against guessing.
- No background janitor in `MemoryStore`; `Sweep` is explicit.

## Risks

- Middleware saving on every request adds one store write per request that
  carries a session. That is the cost of sliding expiration; an external store
  can debounce it behind the same contract.
- Cookie writes that happen after the response started are lost. Documentation
  puts session changes before the response, and `Rotate` is the intended entry
  point after login.
- `MemoryStore` grows with live sessions only; expired records are removed on
  access or `Sweep`.
