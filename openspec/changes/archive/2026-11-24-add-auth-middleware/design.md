# Design

## Password hashing

`HashPassword` uses argon2id from `golang.org/x/crypto/argon2` with a 16-byte
random salt, 64 MiB of memory, three iterations, two lanes, and a 32-byte key.
The result is a PHC string:

```
$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
```

The parameters travel with the hash, so a later change can verify old hashes.
`VerifyPassword` parses the string, recomputes the key with the stored salt and
parameters, and compares with `crypto/subtle`. A malformed hash returns an
error; a wrong password returns false. Salt and key use unpadded standard
base64, which is the common PHC encoding.

Argon2id is a third direct dependency, isolated in `auth`. Applications that do
not import it do not pay for it at link time beyond the module graph.

## User and session

`User` is a small JSON-friendly value: `ID`, `Name`, and optional `Roles`.
`Login` rotates the session id, then stores the marshaled user under one
reserved key. Rotation prevents session fixation; a JSON string keeps the value
stable even when a durable store round-trips through JSON, where a struct would
come back as a map.

`Logout` destroys the session, which deletes the record and clears the cookie.
`UserFromSession` and `UserFromRequest` decode the stored string and report
false when it is absent or malformed. A malformed value is treated as
unauthenticated rather than an error, because the gate must fail closed.

## RequireUser

`RequireUser` is a standard middleware factory. It reads the session from the
request, checks for a user, and either calls the next handler, redirects with
303 to a configured path, or invokes an unauthorized handler. The default
unauthorized response is 401 with a short text body. A configured redirect wins
over the unauthorized handler.

Middleware order matters: the session manager must be registered before
`RequireUser`, so the session is already in the request context. The example
shows a global gate with an explicit login-path exemption.

## Example

`examples/auth` is a single small application in the root module:

- `GET /login` renders a form with a hard-coded demo username and a password
  from `AUTH_PASSWORD` (default `stargo-demo`).
- `POST /login` reads the form, verifies the argon2id hash, calls `auth.Login`,
  and redirects to `/`.
- `GET /` is behind `RequireUser` and shows the user name plus a logout form.
- `POST /logout` calls `auth.Logout` and redirects to `/login`.

The user database is one in-memory demo account; the change is about the
helpers, not a registration flow. Tests cover the redirect, a wrong password,
login, reuse of the session cookie, and logout.

## Non-goals

- No registration, email verification, password reset, or user store.
- No CSRF token. The login form and logout are same-site POSTs, and
  `SameSite=Lax` refuses cross-site POST cookies; a dedicated token remains a
  later change.
- No role middleware beyond the optional `Roles` field.
- No flash messages.
- No rate limiting or account lockout.

## Risks

- Argon2id at 64 MiB per verification is intentionally expensive. A login
  endpoint should be rate limited in production; the package exposes no global
  limiter.
- A session value is JSON, so `Roles` and `Name` must stay serializable.
- The example's single account is not production-shaped and is documented as a
  demo.
